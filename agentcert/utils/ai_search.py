"""
This module provides an async interface for Azure AI Search operations.
It supports index creation, schema definition, chunk ingestion, semantic search,
hybrid search, and document deletion with filters.
"""

import asyncio
import os
from typing import Any, Dict, List, Optional, Union

from azure.core.credentials import AzureKeyCredential
from azure.identity import DefaultAzureCredential
from azure.search.documents import SearchClient
from azure.search.documents.aio import SearchClient as AsyncSearchClient
from azure.search.documents.indexes import SearchIndexClient
from azure.search.documents.indexes.aio import (
    SearchIndexClient as AsyncSearchIndexClient,
)
from azure.search.documents.indexes.models import (
    HnswAlgorithmConfiguration,
    SearchableField,
    SearchField,
    SearchFieldDataType,
    SearchIndex,
    SemanticConfiguration,
    SemanticField,
    SemanticPrioritizedFields,
    SemanticSearch,
    SimpleField,
    VectorSearch,
    VectorSearchProfile,
)
from azure.search.documents.models import VectorizedQuery
from utils.embedding import OpenAIEmbedding
from utils.setup_logging import logger


class AzureAISearch:
    """
    Async class for Azure AI Search operations including index management,
    document ingestion, and search capabilities (semantic and hybrid).
    """

    def __init__(
        self,
        config: dict,
        credential: Optional[DefaultAzureCredential] = None,
        embedding_client: Optional[OpenAIEmbedding] = None,
    ):
        """
        Initialize the AISearch client.

        Args:
            config: Configuration dictionary for AISearch settings.
            credential: Optional DefaultAzureCredential for Azure AD authentication.
        """
        self.config = config
        self.endpoint = os.getenv("AZURE_AI_SEARCH_ENDPOINT")
        self.index_name = self.config.get("ai_search", {}).get(
            "index_name", "documents-index"
        )
        self._api_key = os.getenv("AZURE_AI_SEARCH_API_KEY")

        # Set up credentials - prefer API key if provided, otherwise use DefaultAzureCredential

        if credential:
            self._credential = credential
        elif self._api_key:
            self._credential = AzureKeyCredential(self._api_key)
        else:
            self._credential = DefaultAzureCredential()

        # Initialize clients (will be created lazily for async operations)
        self._index_client: Optional[AsyncSearchIndexClient] = None
        self._search_client: Optional[AsyncSearchClient] = None
        self.embedding_client: OpenAIEmbedding = embedding_client or OpenAIEmbedding(
            config=config
        )

        logger.info(f"AISearch initialized for index: {self.index_name}")

    async def _get_index_client(self) -> AsyncSearchIndexClient:
        """Get or create the async index client."""
        if self._index_client is None:
            self._index_client = AsyncSearchIndexClient(
                endpoint=self.endpoint,
                credential=self._credential,
            )
        return self._index_client

    async def _get_search_client(
        self, index_name: Optional[str] = None
    ) -> AsyncSearchClient:
        """Get or create the async search client."""
        if self._search_client is None:
            self._search_client = AsyncSearchClient(
                endpoint=self.endpoint,
                index_name=index_name or self.index_name,
                credential=self._credential,
            )
        return self._search_client

    def define_index_schema(
        self,
        vector_dimensions: int = 1536,
        vector_config_name: str = "my-vector-config",
        hnsw_config_name: str = "my-hnsw-config",
        semantic_config_name: str = "my-semantic-config",
        custom_fields: Optional[List[SearchField]] = None,
    ) -> SearchIndex:
        """
        Define the index schema with vector search and semantic configuration.

        Args:
            vector_dimensions: Dimensions of the embedding vector (default: 1536 for text-embedding-3-small).
            vector_config_name: Name for the vector search profile.
            hnsw_config_name: Name for the HNSW algorithm configuration.
            semantic_config_name: Name for the semantic configuration.
            custom_fields: Optional list of additional custom fields to include.

        Returns:
            SearchIndex: The configured search index schema.
        """
        # Define default fields for the index
        fields = [
            SimpleField(
                name="id",
                type=SearchFieldDataType.String,
                key=True,
                filterable=True,
                sortable=True,
            ),
            SearchableField(
                name="content",
                type=SearchFieldDataType.String,
                searchable=True,
                analyzer_name="standard.lucene",
            ),
            SearchableField(
                name="title",
                type=SearchFieldDataType.String,
                searchable=True,
                filterable=True,
                sortable=True,
            ),
            SimpleField(
                name="source",
                type=SearchFieldDataType.String,
                filterable=True,
                sortable=True,
            ),
            SimpleField(
                name="topic",
                type=SearchFieldDataType.String,
                filterable=True,
                sortable=True,
            ),
            SimpleField(
                name="language",
                type=SearchFieldDataType.String,
                filterable=True,
                sortable=True,
            ),
            SimpleField(
                name="metadata",
                type=SearchFieldDataType.String,
                filterable=False,
            ),
            SimpleField(
                name="created_at",
                type=SearchFieldDataType.DateTimeOffset,
                filterable=True,
                sortable=True,
            ),
            SearchField(
                name="content_vector",
                type=SearchFieldDataType.Collection(SearchFieldDataType.Single),
                searchable=True,
                vector_search_dimensions=vector_dimensions,
                vector_search_profile_name=vector_config_name,
            ),
        ]

        # Add custom fields if provided
        if custom_fields:
            fields.extend(custom_fields)

        # Configure vector search
        vector_search = VectorSearch(
            algorithms=[
                HnswAlgorithmConfiguration(name=hnsw_config_name),
            ],
            profiles=[
                VectorSearchProfile(
                    name=vector_config_name,
                    algorithm_configuration_name=hnsw_config_name,
                ),
            ],
        )

        # Configure semantic search
        semantic_config = SemanticConfiguration(
            name=semantic_config_name,
            prioritized_fields=SemanticPrioritizedFields(
                title_field=SemanticField(field_name="title"),
                content_fields=[SemanticField(field_name="content")],
                keywords_fields=[
                    SemanticField(field_name="topic"),
                    SemanticField(field_name="source"),
                ],
            ),
        )

        semantic_search = SemanticSearch(configurations=[semantic_config])

        # Create the index schema
        index = SearchIndex(
            name=self.index_name,
            fields=fields,
            vector_search=vector_search,
            semantic_search=semantic_search,
        )

        logger.info(f"Index schema defined for: {self.index_name}")
        return index

    async def create_index(
        self,
        index_schema: Optional[SearchIndex] = None,
        vector_dimensions: int = 1536,
        overwrite: bool = False,
        index_name: Optional[str] = None,
    ) -> SearchIndex:
        """
        Create a search index with the specified schema.

        Args:
            index_schema: Optional pre-defined index schema. If not provided,
                         a default schema will be created.
            vector_dimensions: Dimensions of the embedding vector (used if no schema provided).
            overwrite: If True, delete existing index and create new one.

        Returns:
            SearchIndex: The created search index.
        """
        try:
            index_client = await self._get_index_client()

            if not index_name:
                index_name = self.index_name

            # Check if index exists
            existing_indexes = []
            async for idx in index_client.list_indexes():
                existing_indexes.append(idx.name)

            if index_name in existing_indexes:
                if overwrite:
                    logger.info(f"Deleting existing index: {index_name}")
                    await index_client.delete_index(index_name)
                else:
                    logger.info(f"Index already exists: {index_name}")
                    return await index_client.get_index(index_name)

            # Create schema if not provided
            if index_schema is None:
                index_schema = self.define_index_schema(
                    vector_dimensions=vector_dimensions
                )

            # Create the index
            result = await index_client.create_index(index_schema)
            logger.info(f"Index created successfully: {index_name}")
            return result

        except Exception as e:
            logger.error(f"Error creating index {index_name}: {str(e)}")
            raise

    async def ingest_chunks(
        self,
        chunks: List[Dict[str, Any]],
        batch_size: int = 100,
        index_name: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Ingest document chunks into the search index.

        Args:
            chunks: List of document chunks to ingest. Each chunk should contain:
                   - id: Unique identifier
                   - content: Text content
                   - content_vector: Embedding vector
                   - Other optional fields (title, source, metadata, etc.)
            batch_size: Number of documents to upload per batch.

        Returns:
            Dict containing upload results with success and failure counts.
        """
        try:
            search_client = await self._get_search_client(index_name=index_name)

            total_chunks = len(chunks)
            successful = 0
            failed = 0
            errors = []

            # Process in batches
            for i in range(0, total_chunks, batch_size):
                batch = chunks[i : i + batch_size]

                try:
                    result = await search_client.upload_documents(documents=batch)

                    for doc_result in result:
                        if doc_result.succeeded:
                            successful += 1
                        else:
                            failed += 1
                            errors.append(
                                {
                                    "key": doc_result.key,
                                    "error": doc_result.error_message,
                                }
                            )

                    logger.info(
                        f"Batch {i // batch_size + 1}: Uploaded {len(batch)} chunks"
                    )

                except Exception as batch_error:
                    logger.error(f"Batch upload error: {str(batch_error)}")
                    failed += len(batch)
                    errors.append(
                        {"batch": i // batch_size + 1, "error": str(batch_error)}
                    )

            result_summary = {
                "total": total_chunks,
                "successful": successful,
                "failed": failed,
                "errors": errors if errors else None,
            }

            logger.info(
                f"Ingestion complete: {successful}/{total_chunks} chunks successful"
            )
            return result_summary

        except Exception as e:
            logger.error(f"Error ingesting chunks: {str(e)}")
            raise

    async def semantic_search(
        self,
        query: str,
        query_vector: List[float] = None,
        top_k: int = 5,
        filters: Optional[str] = None,
        select_fields: Optional[List[str]] = None,
        semantic_config_name: str = "my-semantic-config",
        index_name: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """
        Perform semantic (vector) search on the index.

        Args:
            query: The search query text.
            query_vector: The embedding vector of the query.
            top_k: Number of results to return.
            filters: Optional OData filter expression.
            select_fields: Optional list of fields to return in results.
            semantic_config_name: Name of the semantic configuration to use.

        Returns:
            List of search results with scores and document data.
        """
        try:
            search_client = await self._get_search_client(index_name=index_name)

            if query_vector is None:
                query_vector = await self.embedding_client.embed_text(query)

            # Create vector query
            vector_query = VectorizedQuery(
                vector=query_vector,
                k=top_k,
                fields="content_vector",
            )

            # Default fields to select
            if select_fields is None:
                select_fields = self.config.get("ai_search", {}).get(
                    "select_fields", []
                )

            # Execute search
            results = await search_client.search(
                search_text=None,  # Pure vector search
                vector_queries=[vector_query],
                filter=filters,
                select=select_fields,
                top=top_k,
                query_type="semantic",
                semantic_configuration_name=semantic_config_name,
            )

            # Process results
            search_results = []
            async for result in results:
                doc = {
                    "score": result.get("@search.score"),
                    "reranker_score": result.get("@search.reranker_score"),
                }
                for field in select_fields:
                    doc[field] = result.get(field)
                search_results.append(doc)

            logger.info(f"Semantic search returned {len(search_results)} results")
            return search_results

        except Exception as e:
            logger.error(f"Error in semantic search: {str(e)}")
            raise

    async def hybrid_search(
        self,
        query: str,
        query_vector: List[float] = None,
        top_k: int = 5,
        filters: Optional[str] = None,
        select_fields: Optional[List[str]] = None,
        semantic_config_name: str = "my-semantic-config",
        use_semantic_reranker: bool = True,
        index_name: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """
        Perform hybrid search combining keyword and vector search with optional semantic reranking.

        Args:
            query: The search query text for keyword matching.
            query_vector: The embedding vector of the query for vector search.
            top_k: Number of results to return.
            filters: Optional OData filter expression.
            select_fields: Optional list of fields to return in results.
            semantic_config_name: Name of the semantic configuration to use.
            use_semantic_reranker: Whether to apply semantic reranking.

        Returns:
            List of search results with scores and document data.
        """
        try:
            search_client = await self._get_search_client(index_name=index_name)

            if query_vector is None:
                query_vector = await self.embedding_client.embed_text(query)

            # Create vector query
            vector_query = VectorizedQuery(
                vector=query_vector,
                k=top_k,
                fields="content_vector",
            )

            # Default fields to select
            if select_fields is None:
                select_fields = self.config.get("ai_search", {}).get(
                    "select_fields", []
                )

            # Configure query type based on semantic reranker usage
            query_type = "semantic" if use_semantic_reranker else "simple"

            # Execute hybrid search
            search_params = {
                "search_text": query,  # Keyword search
                "vector_queries": [vector_query],  # Vector search
                "filter": filters,
                "select": select_fields,
                "top": top_k,
                "query_type": query_type,
            }

            if use_semantic_reranker:
                search_params["semantic_configuration_name"] = semantic_config_name

            results = await search_client.search(**search_params)

            # Process results
            search_results = []
            async for result in results:
                doc = {
                    "score": result.get("@search.score"),
                    "reranker_score": result.get("@search.reranker_score"),
                }
                for field in select_fields:
                    doc[field] = result.get(field)
                search_results.append(doc)

            logger.info(f"Hybrid search returned {len(search_results)} results")
            return search_results

        except Exception as e:
            logger.error(f"Error in hybrid search: {str(e)}")
            raise

    async def keyword_search(
        self,
        query: str,
        top_k: int = 5,
        filters: Optional[str] = None,
        select_fields: Optional[List[str]] = None,
        semantic_config_name: str = "my-semantic-config",
        use_semantic_reranker: bool = True,
        index_name: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """Perform keyword-only search with optional semantic reranking.

        This avoids query embedding generation (and therefore avoids an extra
        Azure OpenAI embedding call), which can significantly reduce latency.

        Args:
            query: Query string for keyword matching.
            top_k: Number of results to return.
            filters: Optional OData filter.
            select_fields: Optional list of fields to return.
            semantic_config_name: Semantic configuration name.
            use_semantic_reranker: Whether to use semantic reranking.
            index_name: Optional index override.

        Returns:
            List of search results with scores and document data.
        """
        try:
            search_client = await self._get_search_client(index_name=index_name)

            if select_fields is None:
                select_fields = self.config.get("ai_search", {}).get(
                    "select_fields", []
                )

            query_type = "semantic" if use_semantic_reranker else "simple"

            search_params = {
                "search_text": query,
                "filter": filters,
                "select": select_fields,
                "top": top_k,
                "query_type": query_type,
            }

            if use_semantic_reranker:
                search_params["semantic_configuration_name"] = semantic_config_name

            results = await search_client.search(**search_params)

            search_results = []
            async for result in results:
                doc = {
                    "score": result.get("@search.score"),
                    "reranker_score": result.get("@search.reranker_score"),
                }
                for field in select_fields:
                    doc[field] = result.get(field)
                search_results.append(doc)

            logger.info(f"Keyword search returned {len(search_results)} results")
            return search_results

        except Exception as e:
            logger.error(f"Error in keyword search: {str(e)}")
            raise

    async def delete_chunks(
        self,
        filters: Optional[str] = None,
        document_ids: Optional[List[str]] = None,
        index_name: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Delete documents/chunks from the index based on filter or document IDs.

        Args:
            filters: OData filter expression to select documents to delete.
                    Example: "source eq 'document1.pdf'" or "created_at lt 2024-01-01T00:00:00Z"
            document_ids: Optional list of specific document IDs to delete.

        Returns:
            Dict containing deletion results.
        """
        try:
            search_client = await self._get_search_client(index_name=index_name)

            deleted_count = 0
            errors = []

            if document_ids:
                # Delete by specific IDs
                documents_to_delete = [{"id": doc_id} for doc_id in document_ids]
                result = await search_client.delete_documents(
                    documents=documents_to_delete
                )

                for doc_result in result:
                    if doc_result.succeeded:
                        deleted_count += 1
                    else:
                        errors.append(
                            {"key": doc_result.key, "error": doc_result.error_message}
                        )

            elif filters:
                # First, search for documents matching the filter
                results = await search_client.search(
                    search_text="*",
                    filter=filters,
                    select=["id"],
                    top=1000,  # Process in batches of 1000
                )

                ids_to_delete = []
                async for result in results:
                    ids_to_delete.append(result["id"])

                if ids_to_delete:
                    # Delete found documents
                    documents_to_delete = [{"id": doc_id} for doc_id in ids_to_delete]
                    delete_result = await search_client.delete_documents(
                        documents=documents_to_delete
                    )

                    for doc_result in delete_result:
                        if doc_result.succeeded:
                            deleted_count += 1
                        else:
                            errors.append(
                                {
                                    "key": doc_result.key,
                                    "error": doc_result.error_message,
                                }
                            )
            else:
                raise ValueError("Either 'filters' or 'document_ids' must be provided")

            result_summary = {
                "deleted_count": deleted_count,
                "errors": errors if errors else None,
            }

            logger.info(f"Deletion complete: {deleted_count} chunks deleted")
            return result_summary

        except Exception as e:
            logger.error(f"Error deleting chunks: {str(e)}")
            raise

    async def if_index_exists(self, index_name: Optional[str] = None) -> bool:
        """
        Check if the specified index exists.

        Args:
            index_name: Name of the index to check. If None, uses the default index name.
        """
        try:
            if not index_name:
                index_name = self.index_name
            index_client = await self._get_index_client()
            existing_indexes = []
            async for idx in index_client.list_index_names():
                existing_indexes.append(idx)
            exists = index_name in existing_indexes
            logger.info(f"Index '{index_name}' existence check: {exists}")
            return exists
        except Exception as e:
            logger.error(f"Error checking index existence: {str(e)}")
            raise

    async def get_index_stats(self, index_name: Optional[str] = None) -> Dict[str, Any]:
        """
        Get statistics about the current index.

        Returns:
            Dict containing index statistics.
        """
        if not index_name:
            index_name = self.index_name
        try:
            index_client = await self._get_index_client()
            index = await index_client.get_index(index_name)

            stats = {
                "name": index.name,
                "fields_count": len(index.fields),
                "fields": [f.name for f in index.fields],
                "vector_search_profiles": (
                    [p.name for p in index.vector_search.profiles]
                    if index.vector_search
                    else []
                ),
                "semantic_configurations": (
                    [c.name for c in index.semantic_search.configurations]
                    if index.semantic_search
                    else []
                ),
            }

            logger.info(f"Retrieved stats for index: {index_name}")
            return stats

        except Exception as e:
            logger.error(f"Error getting index stats: {str(e)}")
            raise

    async def delete_index(self) -> bool:
        """
        Delete the current index.

        Returns:
            bool: True if deletion was successful.
        """
        try:
            index_client = await self._get_index_client()
            await index_client.delete_index(self.index_name)
            logger.info(f"Index deleted: {self.index_name}")
            return True

        except Exception as e:
            logger.error(f"Error deleting index: {str(e)}")
            raise

    async def close(self):
        """Close all client connections."""
        try:
            if self._search_client:
                await self._search_client.close()
                self._search_client = None

            if self._index_client:
                await self._index_client.close()
                self._index_client = None

            # Close the embedding client as well
            if self.embedding_client:
                await self.embedding_client.close()

            logger.info("AISearch clients closed")

        except Exception as e:
            logger.error(f"Error closing clients: {str(e)}")

    async def __aenter__(self):
        """Async context manager entry."""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit."""
        await self.close()


if __name__ == "__main__":
    import asyncio
    import json
    import time
    import uuid

    async def main():
        with open("configs/config.json", "r") as f:
            config = json.load(f)

        async with AzureAISearch(config=config) as ai_search:
            # Create index
            # if not await ai_search.if_index_exists():
            await ai_search.create_index(overwrite=False)

            branch_info = """
            sample data to be ingested into the search index.
            """

            # Ingest sample chunks
            sample_chunks = [
                {
                    "id": uuid.uuid4().hex,
                    "content": branch_info,
                    "title": "Sample data",
                    "source": "agent report",
                    "topic": "information",
                    "created_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                    "language": "English",
                    "content_vector": await ai_search.embedding_client.embed_text(
                        branch_info
                    ),
                }
            ]
            await ai_search.delete_chunks(filters="source eq 'agent report'")
            await ai_search.ingest_chunks(sample_chunks)

            # Perform a hybrid search
            results = await ai_search.hybrid_search(
                query="sample data",
                top_k=2,
            )
            print("Search Results:", results)

    asyncio.run(main())
