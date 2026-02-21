"""
MongoDB utility module for AgentCert metrics storage.
Provides async/sync MongoDB client with Atlas Vector Search support.
"""

import json
import os
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional, Union

from motor.motor_asyncio import AsyncIOMotorClient, AsyncIOMotorDatabase
from pydantic import BaseModel
from pymongo import ASCENDING, DESCENDING, MongoClient
from pymongo.errors import ConnectionFailure, OperationFailure
from pymongo.operations import SearchIndexModel
from utils.setup_logging import logger


class MongoDBConfig:
    """MongoDB configuration loaded from configs.json."""

    def __init__(self, config: Optional[Dict[str, Any]] = None):
        """Load MongoDB configuration"""
        self._load_config(config)

    def _load_config(self, config) -> None:
        """Load configuration from JSON file."""
        try:
            mongodb_config = config.get("mongodb", {})

            # Connection settings
            self.connection_string = mongodb_config.get(
                "connection_string_env", os.getenv("MONGODB_CONNECTION_STRING")
            )
            self.database_name = mongodb_config.get("database", "agentcert")

            # Collection names
            collections = mongodb_config.get("collections", {})
            self.metrics_collection = collections.get("metrics", "agent_run_metrics")
            self.quantitative_collection = collections.get(
                "quantitative", "llm_quantitative_extractions"
            )
            self.qualitative_collection = collections.get(
                "qualitative", "llm_qualitative_extractions"
            )

            # Vector search settings
            vector_config = mongodb_config.get("vector_search", {})
            self.vector_index_name = vector_config.get(
                "index_name", "metrics_vector_index"
            )
            self.embedding_field = vector_config.get("embedding_field", "embedding")
            self.embedding_dimensions = vector_config.get("dimensions", 1536)
            self.similarity_metric = vector_config.get("similarity", "cosine")
            self.num_candidates = vector_config.get("num_candidates", 100)
            self.vector_limit = vector_config.get("limit", 10)

        except FileNotFoundError:
            logger.warning(
                f"Config file not found at {self.config_path}, using defaults"
            )
            self._set_defaults()
        except json.JSONDecodeError as e:
            logger.error(f"Error parsing config file: {e}")
            self._set_defaults()

    def _set_defaults(self) -> None:
        """Set default configuration values."""
        self.connection_string = os.getenv(
            "MONGODB_CONNECTION_STRING", "mongodb://localhost:27017"
        )
        self.database_name = "agentcert"
        self.metrics_collection = "agent_run_metrics"
        self.quantitative_collection = "llm_quantitative_extractions"
        self.qualitative_collection = "llm_qualitative_extractions"
        self.vector_index_name = "metrics_vector_index"
        self.embedding_field = "embedding"
        self.embedding_dimensions = 1536
        self.similarity_metric = "cosine"
        self.num_candidates = 100
        self.vector_limit = 10


class MongoDBClient:
    """
    MongoDB client with async support and Atlas Vector Search capabilities.

    Supports storing LLMQuantitativeExtraction and LLMQualitativeExtraction
    with optional vector embeddings for semantic search.
    """

    def __init__(self, config: Optional[MongoDBConfig] = None):
        """
        Initialize MongoDB client.

        Args:
            config: Optional MongoDBConfig. If not provided, loads from configs.json.
        """
        self.config = config or MongoDBConfig()
        self._sync_client: Optional[MongoClient] = None
        self._async_client: Optional[AsyncIOMotorClient] = None
        self._sync_db: Optional[Any] = None
        self._async_db: Optional[AsyncIOMotorDatabase] = None

    # ==================== CONNECTION MANAGEMENT ====================

    def _get_sync_client(self) -> MongoClient:
        """Get or create synchronous MongoDB client (lazy initialization)."""
        if self._sync_client is None:
            self._sync_client = MongoClient(self.config.connection_string)
            self._sync_db = self._sync_client[self.config.database_name]
            logger.info(f"Connected to MongoDB (sync): {self.config.database_name}")
        return self._sync_client

    def _get_async_client(self) -> AsyncIOMotorClient:
        """Get or create asynchronous MongoDB client (lazy initialization)."""
        if self._async_client is None:
            self._async_client = AsyncIOMotorClient(self.config.connection_string)
            self._async_db = self._async_client[self.config.database_name]
            logger.info(f"Connected to MongoDB (async): {self.config.database_name}")
        return self._async_client

    @property
    def sync_db(self):
        """Get synchronous database reference."""
        self._get_sync_client()
        return self._sync_db

    @property
    def async_db(self) -> AsyncIOMotorDatabase:
        """Get asynchronous database reference."""
        self._get_async_client()
        return self._async_db

    def close(self) -> None:
        """Close synchronous client connection."""
        if self._sync_client:
            self._sync_client.close()
            self._sync_client = None
            self._sync_db = None
            logger.info("MongoDB sync connection closed")

    async def close_async(self) -> None:
        """Close asynchronous client connection."""
        if self._async_client:
            self._async_client.close()
            self._async_client = None
            self._async_db = None
            logger.info("MongoDB async connection closed")

    def health_check(self) -> bool:
        """Check if MongoDB connection is healthy."""
        try:
            self._get_sync_client()
            self._sync_client.admin.command("ping")
            return True
        except ConnectionFailure as e:
            logger.error(f"MongoDB health check failed: {e}")
            return False

    async def health_check_async(self) -> bool:
        """Check if MongoDB connection is healthy (async)."""
        try:
            self._get_async_client()
            await self._async_client.admin.command("ping")
            return True
        except ConnectionFailure as e:
            logger.error(f"MongoDB async health check failed: {e}")
            return False

    # ==================== COLLECTION INITIALIZATION ====================

    def initialize_collections(self) -> Dict[str, bool]:
        """
        Initialize metrics collection with proper indexes.
        All data (quantitative and qualitative) is stored in a single collection
        as nested documents for each agent run.
        Returns:
            Dict mapping collection name to success status.
        """
        results = {}

        # Create collection if it doesn't exist
        existing_collections = self.sync_db.list_collection_names()
        if self.config.metrics_collection not in existing_collections:
            self.sync_db.create_collection(self.config.metrics_collection)

            # Initialize combined metrics collection (single source of truth)
            results["metrics"] = self._init_metrics_collection()

        else:
            logger.info("Metrics collection already exists, skipping initialization.")

        return results

    def _init_metrics_collection(self) -> bool:
        """Initialize combined metrics collection with indexes."""
        try:
            collection = self.sync_db[self.config.metrics_collection]

            # Create indexes aligned with LLMQuantitativeExtraction / LLMQualitativeExtraction fields
            collection.create_index(
                [("quantitative.experiment_id", ASCENDING)], unique=True, sparse=True
            )
            collection.create_index(
                [
                    ("quantitative.fault_namespace", ASCENDING),
                    ("quantitative.fault_target_service", ASCENDING),
                ]
            )
            collection.create_index([("quantitative.fault_type", ASCENDING)])
            collection.create_index([("qualitative.reasoning_score", DESCENDING)])
            collection.create_index([("qualitative.rai_check_status", ASCENDING)])
            collection.create_index(
                [("qualitative.acceptance_criteria_met", ASCENDING)]
            )
            collection.create_index(
                [("qualitative.security_compliance_status", ASCENDING)]
            )
            collection.create_index([("extraction_timestamp", DESCENDING)])

            logger.info(f"Initialized collection: {self.config.metrics_collection}")
            return True
        except Exception as e:
            logger.error(f"Failed to initialize metrics collection: {e}")
            return False

    def create_vector_search_index(self, collection_name: Optional[str] = None) -> bool:
        """
        Create Atlas Vector Search index on a collection.

        Note: This requires MongoDB Atlas with Vector Search enabled.
        For local MongoDB, vector search is not available.

        Args:
            collection_name: Collection to create index on. Defaults to metrics collection.

        Returns:
            True if index created successfully.
        """
        collection_name = collection_name or self.config.metrics_collection

        try:
            collection = self.sync_db[collection_name]

            # Define vector search index
            search_index_model = SearchIndexModel(
                definition={
                    "fields": [
                        {
                            "type": "vector",
                            "path": self.config.embedding_field,
                            "numDimensions": self.config.embedding_dimensions,
                            "similarity": self.config.similarity_metric,
                        },
                        {
                            "type": "filter",
                            "path": "quantitative.fault_namespace",
                        },
                        {
                            "type": "filter",
                            "path": "quantitative.fault_type",
                        },
                    ]
                },
                name=self.config.vector_index_name,
                type="vectorSearch",
            )

            # Create the search index
            result = collection.create_search_index(model=search_index_model)
            logger.info(
                f"Created vector search index '{self.config.vector_index_name}' on {collection_name}"
            )
            return True

        except OperationFailure as e:
            if "already exists" in str(e):
                logger.info(f"Vector search index already exists on {collection_name}")
                return True
            logger.error(f"Failed to create vector search index: {e}")
            return False
        except Exception as e:
            logger.error(f"Vector search index creation error: {e}")
            return False

    # ==================== CRUD OPERATIONS ====================

    def _prepare_document(
        self,
        data: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
    ) -> Dict[str, Any]:
        """Prepare a document for insertion."""
        if isinstance(data, BaseModel):
            doc = data.model_dump()
        else:
            doc = dict(data)

        # Add metadata
        doc["extraction_timestamp"] = datetime.now(timezone.utc)

        # Add embedding if provided
        if embedding:
            doc[self.config.embedding_field] = embedding

        return doc

    # --- Quantitative Extraction ---

    def insert_quantitative(
        self,
        data: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
    ) -> str:
        """
        Insert a LLMQuantitativeExtraction document.

        Args:
            data: Pydantic model or dict with quantitative data.
            embedding: Optional vector embedding for semantic search.

        Returns:
            Inserted document ID.
        """
        collection = self.sync_db[self.config.quantitative_collection]
        doc = self._prepare_document(data, embedding)
        result = collection.insert_one(doc)
        logger.debug(f"Inserted quantitative extraction: {result.inserted_id}")
        return str(result.inserted_id)

    async def insert_quantitative_async(
        self,
        data: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
    ) -> str:
        """Insert a LLMQuantitativeExtraction document (async)."""
        collection = self.async_db[self.config.quantitative_collection]
        doc = self._prepare_document(data, embedding)
        result = await collection.insert_one(doc)
        logger.debug(f"Inserted quantitative extraction: {result.inserted_id}")
        return str(result.inserted_id)

    def find_quantitative_by_experiment_id(
        self, experiment_id: str
    ) -> Optional[Dict[str, Any]]:
        """Find quantitative extraction by experiment ID."""
        collection = self.sync_db[self.config.quantitative_collection]
        return collection.find_one({"experiment_id": experiment_id})

    async def find_quantitative_by_experiment_id_async(
        self, experiment_id: str
    ) -> Optional[Dict[str, Any]]:
        """Find quantitative extraction by experiment ID (async)."""
        collection = self.async_db[self.config.quantitative_collection]
        return await collection.find_one({"experiment_id": experiment_id})

    # --- Qualitative Extraction ---

    def insert_qualitative(
        self,
        data: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
    ) -> str:
        """
        Insert a LLMQualitativeExtraction document.

        Args:
            data: Pydantic model or dict with qualitative data.
            embedding: Optional vector embedding for semantic search.

        Returns:
            Inserted document ID.
        """
        collection = self.sync_db[self.config.qualitative_collection]
        doc = self._prepare_document(data, embedding)
        result = collection.insert_one(doc)
        logger.debug(f"Inserted qualitative extraction: {result.inserted_id}")
        return str(result.inserted_id)

    async def insert_qualitative_async(
        self,
        data: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
    ) -> str:
        """Insert a LLMQualitativeExtraction document (async)."""
        collection = self.async_db[self.config.qualitative_collection]
        doc = self._prepare_document(data, embedding)
        result = await collection.insert_one(doc)
        logger.debug(f"Inserted qualitative extraction: {result.inserted_id}")
        return str(result.inserted_id)

    def find_metrics_by_experiment_id(
        self, experiment_id: str
    ) -> Optional[Dict[str, Any]]:
        """Find combined metrics by experiment ID in the metrics collection."""
        collection = self.sync_db[self.config.metrics_collection]
        return collection.find_one({"quantitative.experiment_id": experiment_id})

    async def find_metrics_by_experiment_id_async(
        self, experiment_id: str
    ) -> Optional[Dict[str, Any]]:
        """Find combined metrics by experiment ID (async)."""
        collection = self.async_db[self.config.metrics_collection]
        return await collection.find_one({"quantitative.experiment_id": experiment_id})

    # --- Combined Metrics ---

    def insert_metrics(
        self,
        quantitative: Union[BaseModel, Dict[str, Any]],
        qualitative: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> str:
        """
        Insert combined quantitative and qualitative metrics.

        Args:
            quantitative: Quantitative metrics data.
            qualitative: Qualitative metrics data.
            embedding: Optional vector embedding.
            metadata: Optional additional metadata.

        Returns:
            Inserted document ID.
        """
        collection = self.sync_db[self.config.metrics_collection]

        quant_doc = (
            quantitative.model_dump()
            if isinstance(quantitative, BaseModel)
            else dict(quantitative)
        )
        qual_doc = (
            qualitative.model_dump()
            if isinstance(qualitative, BaseModel)
            else dict(qualitative)
        )

        if not quant_doc.get("experiment_id"):
            quant_doc["experiment_id"] = str(uuid.uuid4())

        doc = {
            "quantitative": quant_doc,
            "qualitative": qual_doc,
            "extraction_timestamp": datetime.now(timezone.utc),
        }

        if embedding:
            doc[self.config.embedding_field] = embedding

        if metadata:
            doc["metadata"] = metadata

        result = collection.insert_one(doc)
        logger.debug(f"Inserted combined metrics: {result.inserted_id}")
        return str(result.inserted_id)

    async def insert_metrics_async(
        self,
        quantitative: Union[BaseModel, Dict[str, Any]],
        qualitative: Union[BaseModel, Dict[str, Any]],
        embedding: Optional[List[float]] = None,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> str:
        """Insert combined metrics (async)."""
        collection = self.async_db[self.config.metrics_collection]

        quant_doc = (
            quantitative.model_dump()
            if isinstance(quantitative, BaseModel)
            else dict(quantitative)
        )
        qual_doc = (
            qualitative.model_dump()
            if isinstance(qualitative, BaseModel)
            else dict(qualitative)
        )

        if not quant_doc.get("experiment_id"):
            quant_doc["experiment_id"] = str(uuid.uuid4())

        doc = {
            "quantitative": quant_doc,
            "qualitative": qual_doc,
            "extraction_timestamp": datetime.now(timezone.utc),
        }

        if embedding:
            doc[self.config.embedding_field] = embedding

        if metadata:
            doc["metadata"] = metadata

        result = await collection.insert_one(doc)
        logger.debug(f"Inserted combined metrics: {result.inserted_id}")
        return str(result.inserted_id)

    # ==================== VECTOR SEARCH ====================

    def vector_search(
        self,
        query_embedding: List[float],
        collection_name: Optional[str] = None,
        filter_query: Optional[Dict[str, Any]] = None,
        limit: Optional[int] = None,
    ) -> List[Dict[str, Any]]:
        """
        Perform vector similarity search using Atlas Vector Search.

        Args:
            query_embedding: Query vector (1536 dimensions for OpenAI embeddings).
            collection_name: Collection to search. Defaults to metrics collection.
            filter_query: Optional MongoDB filter to apply.
            limit: Maximum results to return.

        Returns:
            List of matching documents with similarity scores.
        """
        collection_name = collection_name or self.config.metrics_collection
        limit = limit or self.config.vector_limit
        collection = self.sync_db[collection_name]

        # Build aggregation pipeline for vector search
        pipeline = [
            {
                "$vectorSearch": {
                    "index": self.config.vector_index_name,
                    "path": self.config.embedding_field,
                    "queryVector": query_embedding,
                    "numCandidates": self.config.num_candidates,
                    "limit": limit,
                }
            },
            {"$addFields": {"search_score": {"$meta": "vectorSearchScore"}}},
        ]

        # Add filter if provided
        if filter_query:
            pipeline[0]["$vectorSearch"]["filter"] = filter_query

        # Exclude embedding field from results (can be large)
        pipeline.append({"$project": {self.config.embedding_field: 0}})

        try:
            results = list(collection.aggregate(pipeline))
            logger.debug(f"Vector search returned {len(results)} results")
            return results
        except OperationFailure as e:
            logger.error(f"Vector search failed: {e}")
            return []

    async def vector_search_async(
        self,
        query_embedding: List[float],
        collection_name: Optional[str] = None,
        filter_query: Optional[Dict[str, Any]] = None,
        limit: Optional[int] = None,
    ) -> List[Dict[str, Any]]:
        """Perform vector similarity search (async)."""
        collection_name = collection_name or self.config.metrics_collection
        limit = limit or self.config.vector_limit
        collection = self.async_db[collection_name]

        pipeline = [
            {
                "$vectorSearch": {
                    "index": self.config.vector_index_name,
                    "path": self.config.embedding_field,
                    "queryVector": query_embedding,
                    "numCandidates": self.config.num_candidates,
                    "limit": limit,
                }
            },
            {"$addFields": {"search_score": {"$meta": "vectorSearchScore"}}},
        ]

        if filter_query:
            pipeline[0]["$vectorSearch"]["filter"] = filter_query

        pipeline.append({"$project": {self.config.embedding_field: 0}})

        try:
            cursor = collection.aggregate(pipeline)
            results = await cursor.to_list(length=limit)
            logger.debug(f"Vector search returned {len(results)} results")
            return results
        except OperationFailure as e:
            logger.error(f"Vector search failed: {e}")
            return []

    # ==================== QUERY OPERATIONS ====================

    def find_by_fault_type(
        self,
        fault_type: str,
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        """Find metrics by fault type in the combined metrics collection."""
        collection = self.sync_db[self.config.metrics_collection]

        cursor = (
            collection.find({"quantitative.fault_type": fault_type})
            .sort("extraction_timestamp", DESCENDING)
            .limit(limit)
        )

        return list(cursor)

    def find_by_namespace(
        self, namespace: str, limit: int = 100
    ) -> List[Dict[str, Any]]:
        """Find metrics by Kubernetes namespace in the combined metrics collection."""
        collection = self.sync_db[self.config.metrics_collection]

        cursor = (
            collection.find({"quantitative.fault_namespace": namespace})
            .sort("extraction_timestamp", DESCENDING)
            .limit(limit)
        )

        return list(cursor)

    def get_metrics_summary(self) -> Dict[str, Any]:
        """Get summary statistics for stored metrics."""
        quant_collection = self.sync_db[self.config.quantitative_collection]
        qual_collection = self.sync_db[self.config.qualitative_collection]
        metrics_collection = self.sync_db[self.config.metrics_collection]

        return {
            "quantitative_count": quant_collection.count_documents({}),
            "qualitative_count": qual_collection.count_documents({}),
            "combined_metrics_count": metrics_collection.count_documents({}),
            "collections": {
                "quantitative": self.config.quantitative_collection,
                "qualitative": self.config.qualitative_collection,
                "metrics": self.config.metrics_collection,
            },
        }

    async def get_metrics_summary_async(self) -> Dict[str, Any]:
        """Get summary statistics for stored metrics (async)."""
        quant_collection = self.async_db[self.config.quantitative_collection]
        qual_collection = self.async_db[self.config.qualitative_collection]
        metrics_collection = self.async_db[self.config.metrics_collection]

        return {
            "quantitative_count": await quant_collection.count_documents({}),
            "qualitative_count": await qual_collection.count_documents({}),
            "combined_metrics_count": await metrics_collection.count_documents({}),
            "collections": {
                "quantitative": self.config.quantitative_collection,
                "qualitative": self.config.qualitative_collection,
                "metrics": self.config.metrics_collection,
            },
        }


# ==================== CONVENIENCE FUNCTIONS ====================


def get_mongodb_client(config: Optional[MongoDBConfig] = None) -> MongoDBClient:
    """Get a MongoDB client instance."""
    return MongoDBClient(config)


def initialize_mongodb() -> Dict[str, bool]:
    """Initialize MongoDB collections and indexes."""
    client = MongoDBClient()
    try:
        results = client.initialize_collections()
        return results
    finally:
        client.close()


if __name__ == "__main__":
    # Test MongoDB connection, initialization, insert, and delete
    print("Testing MongoDB connection...")

    from utils.load_config import ConfigLoader

    mongo_config = MongoDBConfig(ConfigLoader.load_config())
    client = MongoDBClient(mongo_config)

    try:
        if not client.health_check():
            print("❌ MongoDB connection failed")
            print("Make sure MongoDB is running and MONGODB_CONNECTION_STRING is set")
            exit(1)

        print("✅ MongoDB connection successful")

        # Initialize collections
        print("\nInitializing collections...")
        results = client.initialize_collections()

        for collection, success in results.items():
            status = "✅" if success else "❌"
            print(f"  {status} {collection}")

        # Create sample metrics data
        print("\n📝 Creating sample metrics document...")

        experiment_id = (
            f"exp_test_{datetime.now(timezone.utc).strftime('%Y%m%d_%H%M%S')}"
        )

        quantitative_data = {
            "experiment_id": experiment_id,
            "fault_injection_time": "2026-02-15T10:00:00Z",
            "agent_fault_detection_time": "2026-02-15T10:00:15Z",
            "agent_fault_mitigation_time": "2026-02-15T10:01:30Z",
            "time_to_detect": 15.0,
            "time_to_mitigate": 90.0,
            "framework_overhead_seconds": 2.5,
            "fault_detected": "Misconfig",
            "trajectory_steps": 12,
            "input_tokens": 5000,
            "output_tokens": 1500,
            "fault_type": "Misconfig",
            "fault_target_service": "payment-service",
            "fault_namespace": "production",
            "tool_calls": [
                {
                    "tool_name": "get_logs",
                    "arguments": {"service": "payment-service"},
                    "was_successful": True,
                    "response_summary": "Retrieved logs",
                    "timestamp": "2026-02-15T10:00:10Z",
                }
            ],
        }

        qualitative_data = {
            "rai_check_status": "Passed",
            "rai_check_notes": "No harmful content detected",
            "trajectory_efficiency_score": 8.5,
            "trajectory_efficiency_notes": "Efficient diagnostic path",
            "security_compliance_status": "Compliant",
            "security_compliance_notes": "No credentials exposed",
            "acceptance_criteria_met": True,
            "acceptance_criteria_notes": "Fault correctly detected and mitigated",
            "response_quality_score": 9.0,
            "response_quality_notes": "Clear and accurate reasoning",
            "reasoning_score": 8,
            "reasoning_judgement": "Strong diagnostic reasoning",
            "known_limitations": ["Could have checked more services"],
            "recommendations": ["Add broader health checks"],
            "agent_summary": "Agent detected misconfig in payment-service and remediated it.",
        }

        metadata = {
            "trace_file": "test_trace.json",
            "total_spans": 12,
            "extraction_token_usage": {
                "input_tokens": 3000,
                "output_tokens": 800,
                "total_tokens": 3800,
            },
        }

        # Insert the metrics document
        doc_id = client.insert_metrics(
            quantitative=quantitative_data,
            qualitative=qualitative_data,
            metadata=metadata,
        )

        print(f"✅ Inserted metrics document with ID: {doc_id}")
        print(f"   Experiment ID: {experiment_id}")

        # Verify the document was inserted using experiment_id lookup
        print("\n🔍 Verifying document insertion...")
        inserted_doc = client.find_metrics_by_experiment_id(experiment_id)

        if inserted_doc:
            print(f"✅ Document found in database")
            print(
                f"   Quantitative metrics: {list(inserted_doc['quantitative'].keys())}"
            )
            print(f"   Qualitative metrics: {list(inserted_doc['qualitative'].keys())}")
            print(f"   Metadata: {inserted_doc.get('metadata', {})}")
        else:
            print("❌ Document not found after insertion")

        # Get metrics summary
        print("\n📊 Metrics summary:")
        summary = client.get_metrics_summary()
        for key, value in summary.items():
            if key != "collections":
                print(f"   {key}: {value}")

        # Delete the test document
        print(f"\n🗑️  Deleting test document (experiment_id: {experiment_id})...")
        collection = client.sync_db[client.config.metrics_collection]
        delete_result = collection.delete_one(
            {"quantitative.experiment_id": experiment_id}
        )

        if delete_result.deleted_count > 0:
            print(
                f"✅ Document deleted successfully (deleted {delete_result.deleted_count} document)"
            )
        else:
            print("❌ Document deletion failed - no documents matched")

        # Verify deletion
        print("\n🔍 Verifying document deletion...")
        deleted_doc = client.find_metrics_by_experiment_id(experiment_id)

        if deleted_doc is None:
            print("✅ Document successfully removed from database")
        else:
            print("❌ Document still exists in database")

        # Final summary
        print("\n📊 Final metrics summary:")
        final_summary = client.get_metrics_summary()
        for key, value in final_summary.items():
            if key != "collections":
                print(f"   {key}: {value}")

        print("\n✅ Test completed successfully!")

    except Exception as e:
        print(f"\n❌ Error during test: {e}")
        import traceback

        traceback.print_exc()

    finally:
        client.close()
        print("\n🔌 MongoDB connection closed")
