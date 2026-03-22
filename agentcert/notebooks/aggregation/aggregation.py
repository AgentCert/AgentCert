"""
Fault-category and overall agent-level metrics aggregation for AgentCert.

Aggregates per-run metrics (stored in MongoDB by metrics_extractor_from_trace.py)
into fault-category level scorecards, then into an overall agent-level certification
scorecard matching the structure defined in mock_aggregated_scorecards.json.

Numeric metrics are aggregated deterministically in code; textual/narrative metrics
are synthesized via an LLM Council.

Reference: AgentCert.wiki/Methodologies/03-Experimentation/3.2-Aggregation.md
"""

import asyncio
import json
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

from utils.azure_openai_util import AzureLLMClient
from utils.load_config import ConfigLoader
from utils.mongodb_util import MongoDBClient, MongoDBConfig
from utils.setup_logging import logger

from .llm_council import LLMCouncil
from .numeric_aggregation import (
    compute_boolean_aggregates,
    compute_derived_rates,
    compute_numeric_aggregates,
)

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

AGGREGATED_SCORECARDS_COLLECTION = "aggregated_scorecards"


# ---------------------------------------------------------------------------
# MongoDB query helpers
# ---------------------------------------------------------------------------


class MetricsQueryService:
    """Handles all MongoDB queries for per-run metric documents."""

    def __init__(self, db_client: MongoDBClient):
        self.db_client = db_client

    def query_runs_by_agent(self, agent_id: str) -> List[Dict[str, Any]]:
        """Query all per-run metric documents for a given agent_id."""
        docs = self.db_client.find_by_agent_id(agent_id)
        logger.info(
            f"Queried {len(docs)} per-run documents for agent_id='{agent_id}'"
        )
        return docs

    def query_runs_by_fault_category(
        self,
        fault_category: str,
        agent_id: Optional[str] = None,
    ) -> List[Dict[str, Any]]:
        """Query per-run metric documents for a fault_category (optionally scoped to agent)."""
        collection = self.db_client.sync_db[self.db_client.config.metrics_collection]
        query: Dict[str, Any] = {"fault_category": fault_category}
        if agent_id:
            query["agent_id"] = agent_id
        docs = list(collection.find(query))
        logger.info(
            f"Queried {len(docs)} per-run documents for fault_category='{fault_category}'"
            + (f", agent_id='{agent_id}'" if agent_id else "")
        )
        return docs

    def get_all_fault_categories(
        self,
        agent_id: Optional[str] = None,
    ) -> List[str]:
        """Return distinct fault_category values in the metrics collection."""
        collection = self.db_client.sync_db[self.db_client.config.metrics_collection]
        filter_query = {"agent_id": agent_id} if agent_id else {}
        categories = collection.distinct("fault_category", filter_query)
        return [c for c in categories if c is not None]


# ---------------------------------------------------------------------------
# Scorecard assembly
# ---------------------------------------------------------------------------


class ScorecardAssembler:
    """Assembles fault-category and certification-level scorecards."""

    @staticmethod
    def assemble_category_scorecard(
        fault_category: str,
        docs: List[Dict[str, Any]],
        numeric_aggs: Dict[str, Dict[str, Any]],
        derived_rates: Dict[str, Optional[float]],
        boolean_aggs: Dict[str, Any],
        textual_aggs: Dict[str, Any],
    ) -> Dict[str, Any]:
        """Assemble all aggregation results into a fault-category scorecard dict."""
        fault_names = set()
        for doc in docs:
            fname = doc.get("fault_name") or doc.get("quantitative", {}).get("injected_fault_name")
            if fname:
                fault_names.add(fname)

        return {
            "fault_category": fault_category,
            "faults_tested": sorted(fault_names),
            "total_runs": len(docs),
            "numeric_metrics": numeric_aggs,
            "derived_metrics": derived_rates,
            "boolean_status_metrics": boolean_aggs,
            "textual_metrics": textual_aggs,
        }

    @staticmethod
    def assemble_final_scorecard(
        category_scorecards: List[Dict[str, Any]],
        agent_id: str = "",
        agent_name: str = "",
        certification_run_id: str = "",
        runs_per_fault: int = 30,
    ) -> Dict[str, Any]:
        """Assemble the final certification scorecard combining all fault-category scorecards."""
        total_runs = sum(sc.get("total_runs", 0) for sc in category_scorecards)
        all_faults = set()
        for sc in category_scorecards:
            all_faults.update(sc.get("faults_tested", []))

        return {
            "agent_id": agent_id,
            "agent_name": agent_name,
            "certification_run_id": certification_run_id,
            "created_at": datetime.now(timezone.utc).isoformat(),
            "total_runs": total_runs,
            "total_faults_tested": len(all_faults),
            "total_fault_categories": len(category_scorecards),
            "runs_per_fault": runs_per_fault,
            "fault_category_scorecards": category_scorecards,
        }


# ---------------------------------------------------------------------------
# MongoDB storage
# ---------------------------------------------------------------------------


class ScorecardStorage:
    """Stores certification scorecards in MongoDB."""

    def __init__(self, db_client: MongoDBClient):
        self.db_client = db_client

    def store(self, scorecard: Dict[str, Any]) -> str:
        """Store the full certification scorecard (upsert on certification_run_id)."""
        collection = self.db_client.sync_db[AGGREGATED_SCORECARDS_COLLECTION]
        cert_run_id = scorecard.get("certification_run_id", "")

        filter_key = (
            {"certification_run_id": cert_run_id}
            if cert_run_id
            else {"agent_id": scorecard.get("agent_id", "")}
        )

        result = collection.replace_one(filter_key, scorecard, upsert=True)

        if result.upserted_id:
            doc_id = str(result.upserted_id)
            logger.info(f"Inserted new certification scorecard: {doc_id}")
        else:
            doc_id = cert_run_id or scorecard.get("agent_id", "")
            logger.info(f"Updated existing certification scorecard: {doc_id}")

        return doc_id


# ---------------------------------------------------------------------------
# Aggregation orchestrator
# ---------------------------------------------------------------------------


class AggregationOrchestrator:
    """Orchestrates the full aggregation pipeline."""

    def __init__(
        self,
        db_client: MongoDBClient,
        llm_client: AzureLLMClient,
    ):
        self.query_service = MetricsQueryService(db_client)
        self.council = LLMCouncil(llm_client)
        self.assembler = ScorecardAssembler()
        self.storage = ScorecardStorage(db_client)

    async def aggregate_fault_category(
        self,
        fault_category: str,
        agent_id: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        Full aggregation pipeline for a single fault category.

        Steps:
        1. Query per-run metrics from MongoDB
        2. Compute numeric aggregates
        3. Compute derived rate metrics
        4. Compute boolean/status aggregates
        5. Synthesize textual metrics via LLM Council
        5b. Synthesize known_limitations & recommendations from aggregated metrics
        6. Assemble the category scorecard
        """
        logger.info(
            f"Starting aggregation for fault_category='{fault_category}'"
            + (f", agent_id='{agent_id}'" if agent_id else "")
        )

        # Step 1: Query
        docs = self.query_service.query_runs_by_fault_category(
            fault_category, agent_id=agent_id
        )
        if not docs:
            logger.warning(f"No per-run documents found for fault_category='{fault_category}'")
            return {
                "fault_category": fault_category,
                "faults_tested": [],
                "total_runs": 0,
                "numeric_metrics": {},
                "derived_metrics": {},
                "boolean_status_metrics": {},
                "textual_metrics": {},
            }

        # Step 2: Numeric aggregates
        numeric_aggs = compute_numeric_aggregates(docs)
        logger.info(f"Computed numeric aggregates for {len(numeric_aggs)} metrics")

        # Step 3: Derived rates
        derived_rates = compute_derived_rates(docs)
        logger.info(f"Computed derived rates: {derived_rates}")

        # Step 4: Boolean aggregates
        boolean_aggs = compute_boolean_aggregates(docs)
        logger.info(f"Computed boolean aggregates: {boolean_aggs}")

        # Step 5: Textual aggregates via LLM Council
        textual_aggs, textual_usage = await self.council.compute_textual_aggregates(
            docs, fault_category
        )
        logger.info(
            f"Completed LLM Council synthesis for {len(textual_aggs)} textual metrics "
            f"(tokens: {textual_usage})"
        )

        # Step 5b: Synthesize known_limitations & recommendations
        fault_names = set()
        for doc in docs:
            fname = doc.get("fault_name") or doc.get("quantitative", {}).get("injected_fault_name")
            if fname:
                fault_names.add(fname)

        synthesis_result, synthesis_usage = await self.council.synthesize_limitations_and_recommendations(
            fault_category=fault_category,
            faults_tested=sorted(fault_names),
            total_runs=len(docs),
            numeric_aggs=numeric_aggs,
            derived_rates=derived_rates,
            boolean_aggs=boolean_aggs,
            textual_aggs=textual_aggs,
        )
        textual_aggs.update(synthesis_result)
        logger.info(
            f"Synthesized known_limitations and recommendations "
            f"(tokens: {synthesis_usage})"
        )

        # Step 6: Assemble category scorecard
        scorecard = self.assembler.assemble_category_scorecard(
            fault_category=fault_category,
            docs=docs,
            numeric_aggs=numeric_aggs,
            derived_rates=derived_rates,
            boolean_aggs=boolean_aggs,
            textual_aggs=textual_aggs,
        )

        logger.info(
            f"Aggregation complete for '{fault_category}': "
            f"{scorecard['total_runs']} runs, "
            f"{len(scorecard['faults_tested'])} fault types"
        )

        return scorecard

    async def aggregate_all(
        self,
        agent_id: str = "",
        agent_name: str = "",
        certification_run_id: str = "",
        runs_per_fault: int = 30,
        store_results: bool = True,
    ) -> Dict[str, Any]:
        """
        Aggregate metrics for all fault categories and produce the final certification scorecard.

        Processes categories sequentially to manage LLM API rate limits.
        """
        categories = self.query_service.get_all_fault_categories(
            agent_id=agent_id or None
        )
        logger.info(f"Found {len(categories)} fault categories: {categories}")

        category_scorecards: List[Dict[str, Any]] = []

        for category in categories:
            scorecard = await self.aggregate_fault_category(
                fault_category=category,
                agent_id=agent_id or None,
            )
            category_scorecards.append(scorecard)

        logger.info(f"Completed aggregation for {len(category_scorecards)} fault categories")

        final_scorecard = self.assembler.assemble_final_scorecard(
            category_scorecards=category_scorecards,
            agent_id=agent_id,
            agent_name=agent_name,
            certification_run_id=certification_run_id,
            runs_per_fault=runs_per_fault,
        )

        if store_results:
            doc_id = self.storage.store(final_scorecard)
            logger.info(f"Certification scorecard stored: {doc_id}")

        return final_scorecard


# ---------------------------------------------------------------------------
# CLI entry point
# ---------------------------------------------------------------------------

async def main():
    """CLI entry point for fault-category aggregation."""
    import argparse

    parser = argparse.ArgumentParser(
        description="Aggregate per-run metrics into fault-category and certification scorecards"
    )
    parser.add_argument(
        "--agent-id", type=str, required=True,
        help="Agent ID to aggregate metrics for",
    )
    parser.add_argument(
        "--agent-name", type=str, required=True,
        help="Agent name for the certification scorecard",
    )
    parser.add_argument(
        "--certification-run-id", type=str, default="",
        help="Optional certification run ID",
    )
    parser.add_argument(
        "--runs-per-fault", type=int, default=30,
        help="Expected number of runs per fault (default: 30)",
    )
    parser.add_argument(
        "--no-store", action="store_true",
        help="Skip storing the scorecard to MongoDB",
    )

    args = parser.parse_args()

    config = ConfigLoader.load_config()
    mongo_config = MongoDBConfig(config)
    db_client = MongoDBClient(mongo_config)
    llm_client = AzureLLMClient(config=config)

    try:
        if not db_client.health_check():
            logger.error("MongoDB connection failed. Ensure MongoDB is running.")
            return

        logger.info(
            f"MongoDB connection successful. "
            f"Starting aggregation for agent_id='{args.agent_id}', agent_name='{args.agent_name}'..."
        )

        orchestrator = AggregationOrchestrator(db_client, llm_client)

        # Verify documents exist
        agent_docs = orchestrator.query_service.query_runs_by_agent(args.agent_id)
        if not agent_docs:
            logger.warning(
                f"No per-run documents found for agent_id='{args.agent_id}'. "
                "Ensure per-run metrics have been extracted with "
                "metrics_extractor_from_trace.py first."
            )
            return

        logger.info(f"Found {len(agent_docs)} per-run documents for agent_id='{args.agent_id}'")

        categories = orchestrator.query_service.get_all_fault_categories(agent_id=args.agent_id)
        if not categories:
            logger.warning(f"No fault categories found for agent_id='{args.agent_id}'.")
            return

        logger.info(f"Found fault categories for agent: {categories}")

        final_scorecard = await orchestrator.aggregate_all(
            agent_id=args.agent_id,
            agent_name=args.agent_name,
            certification_run_id=args.certification_run_id,
            runs_per_fault=args.runs_per_fault,
            store_results=not args.no_store,
        )

        # Print summary
        print("\n" + "=" * 70)
        print("CERTIFICATION SCORECARD SUMMARY")
        print("=" * 70)
        print(f"  Agent: {args.agent_name} ({args.agent_id})")
        print(f"  Total categories: {final_scorecard['total_fault_categories']}")
        print(f"  Total faults tested: {final_scorecard['total_faults_tested']}")
        print(f"  Total runs: {final_scorecard['total_runs']}")

        for sc in final_scorecard.get("fault_category_scorecards", []):
            print(f"\n  Category: {sc['fault_category']}")
            print(f"    Total runs: {sc['total_runs']}")
            print(f"    Faults tested: {', '.join(sc.get('faults_tested', []))}")

            derived = sc.get("derived_metrics", {})
            print(f"    Detection success rate: {derived.get('fault_detection_success_rate')}")
            print(f"    Mitigation success rate: {derived.get('fault_mitigation_success_rate')}")
            print(f"    RAI compliance rate: {derived.get('rai_compliance_rate')}")
            print(f"    Security compliance rate: {derived.get('security_compliance_rate')}")

            num = sc.get("numeric_metrics", {})
            ttd = num.get("time_to_detect", {})
            if ttd.get("median") is not None:
                print(f"    Time to detect (median): {ttd['median']}s")
            ttm = num.get("time_to_mitigate", {})
            if ttm.get("median") is not None:
                print(f"    Time to mitigate (median): {ttm['median']}s")

        print("\n" + "=" * 70)
        print(f"Scorecard stored in MongoDB collection: '{AGGREGATED_SCORECARDS_COLLECTION}'")
        print("=" * 70)

        output_path = "aggregated_scorecard_output.json"
        with open(output_path, "w") as f:
            json.dump(final_scorecard, f, indent=4, default=str)
        print(f"Scorecard also written to: {output_path}")

    except Exception as e:
        logger.error(f"Aggregation failed: {e}")
        import traceback
        traceback.print_exc()

    finally:
        db_client.close()
        await llm_client.close()
        logger.info("Connections closed.")


if __name__ == "__main__":
    asyncio.run(main())
