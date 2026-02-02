# AgentCert Documentation

## Overview

AgentCert is a comprehensive framework for certifying and evaluating IT-Ops AI agents. It extracts both quantitative and qualitative metrics from agent run reports using LLM-based analysis, stores results in MongoDB, and supports Azure AI Search for semantic search capabilities.

---

## Project Structure

```
agentcert/
├── configs/
│   └── configs.json          # Configuration file for all services
├── data/
│   └── app.log               # Any data or log files are stored here
├── data_models/
│   └── metrics_model.py      # Pydantic models for metrics
├── notebooks/
│   └── metrics_extraction/
│       ├── __init__.py       # Package exports
│       └── metrics_extractor.py  # Main metrics extraction module
├── prompts/
│   └── prompt.yml            # Prompt templates (placeholder)
├── utils/
│   ├── azure_openai_util.py  # Azure OpenAI client
│   ├── custom_errors.py      # Custom exception classes
│   ├── embedding.py          # Text embedding utilities
│   ├── file_storage.py       # Azure Blob Storage utilities
│   ├── load_config.py        # Configuration loader
│   ├── mongodb_util.py       # MongoDB client utilities
│   ├── rai_util.py           # Responsible AI content safety
│   └── setup_logging.py      # Logging configuration
├── requirements.txt          # Python dependencies
└── .env.example              # Environment variables template
```

---

## Configuration Files

### configs/configs.json

Central configuration file containing settings for:
- **MongoDB**: Connection string, database name, collection names, vector search settings
- **Storage Connections**: Azure Blob Storage account configuration
- **Models**: Azure OpenAI model configurations (embedding, extraction, reasoning)

---

## Data Models

### data_models/metrics_model.py

Contains Pydantic models for structured data validation and serialization.

| Class | Description |
|-------|-------------|
| `BaseModelWrapper` | Base class providing `get()`, `to_dict()`, and `to_json()` utility methods |
| `RAICheckStatus` | Enum: `PASSED`, `FAILED`, `NOT_EVALUATED` |
| `SecurityComplianceStatus` | Enum: `COMPLIANT`, `NON_COMPLIANT`, `PARTIALLY_COMPLIANT`, `NOT_EVALUATED` |
| `ToolCall` | Model for agent tool call records (tool_name, arguments, response_summary, was_successful, timestamp) |
| `FaultInfo` | Model for fault injection information (fault_type, target_service, namespace) |
| `MetricsExtractionResult` | Result container with success flag, metrics dict, errors, and warnings |
| `LLMQuantitativeExtraction` | Model for quantitative metrics (session_id, namespace, deployment, timing, tokens, tool_calls) |
| `LLMQualitativeExtraction` | Model for qualitative metrics (RAI status, security compliance, reasoning scores, recommendations) |

---

## Utility Modules

### utils/azure_openai_util.py

Azure OpenAI client wrapper using the `agent_framework` library.

| Class | Description |
|-------|-------------|
| `AzureLLMClient` | Main client for Azure OpenAI interactions |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__(config)` | Initialize client with optional configuration dictionary |
| `get_client(config)` | Class method to get/create shared `AzureOpenAIChatClient` instance |
| `get_clients(config)` | Class method to get/create clients for all models in config |
| `_get_or_create_agent(model_name, system_prompt, tools)` | Get or create a `ChatAgent` for the specified model |
| `_convert_messages_to_chat_messages(messages)` | Convert various message formats to `ChatMessage` list |
| `call_llm(model_name, messages, temperature, max_tokens, system_prompt)` | Call the LLM with given parameters, returns `(response_dict, cost)` |
| `with_structured_output(model_name, messages, output_format, ...)` | Call LLM with Pydantic model for structured JSON output |
| `get_chat_completion(model_name, messages, ...)` | Wrapper for `call_llm` with chat completion interface |
| `run_agent(agent_name, messages, system_prompt, tools)` | Run a `ChatAgent` with optional tools |
| `run_agent_stream(agent_name, messages, system_prompt, tools)` | Run agent with streaming response |
| `close()` | Close all client sessions |
| `warmup()` | Class method to warm up the client for reduced first-call latency |

---

### utils/load_config.py

Configuration loading utilities.

| Class | Description |
|-------|-------------|
| `EnvLoader` | Static utility for loading environment variables |
| `ConfigLoader` | Static utility for loading JSON configuration with ENV variable resolution |

#### Methods

| Method | Description |
|--------|-------------|
| `EnvLoader.load_env_vars(variable_name, compulsory)` | Load environment variable, with optional required check |
| `ConfigLoader._resolve_env_values(config_data)` | Recursively resolve `ENV_*` prefixed values to environment variables |
| `ConfigLoader.load_config()` | Load and resolve `configs/configs.json` |

---

### utils/mongodb_util.py

MongoDB client with async support and Atlas Vector Search capabilities.

| Class | Description |
|-------|-------------|
| `MongoDBConfig` | Configuration class loading MongoDB settings from configs.json |
| `MongoDBClient` | Main MongoDB client supporting sync/async operations |

#### MongoDBConfig Methods

| Method | Description |
|--------|-------------|
| `__init__(config)` | Load MongoDB configuration from dict |
| `_load_config(config)` | Parse configuration values |
| `_set_defaults()` | Set default configuration values |

#### MongoDBClient Methods

| Method | Description |
|--------|-------------|
| `_get_sync_client()` | Lazy initialization of synchronous MongoDB client |
| `_get_async_client()` | Lazy initialization of asynchronous MongoDB client |
| `close()` | Close synchronous client connection |
| `close_async()` | Close asynchronous client connection |
| `health_check()` | Check MongoDB connection health (sync) |
| `health_check_async()` | Check MongoDB connection health (async) |
| `initialize_collections()` | Create collections with proper indexes |
| `_init_metrics_collection()` | Initialize combined metrics collection |
| `create_vector_search_index(collection_name)` | Create Atlas Vector Search index |
| `_prepare_document(data, embedding)` | Prepare document for insertion with metadata |
| `insert_quantitative(data, embedding)` | Insert quantitative extraction document |
| `insert_quantitative_async(data, embedding)` | Async version of insert_quantitative |
| `find_quantitative_by_session(session_id)` | Find quantitative extraction by session ID |
| `insert_qualitative(data, embedding)` | Insert qualitative extraction document |
| `insert_qualitative_async(data, embedding)` | Async version of insert_qualitative |
| `find_qualitative_by_session(session_id)` | Find qualitative extraction by session ID |
| `insert_metrics(quantitative, qualitative, session_id, embedding, metadata)` | Insert combined metrics document |
| `insert_metrics_async(...)` | Async version of insert_metrics |
| `vector_search(query_embedding, collection_name, filter_query, limit)` | Perform vector similarity search |
| `vector_search_async(...)` | Async version of vector_search |
| `find_by_accuracy(detection_accuracy, collection_name, limit)` | Find metrics by detection accuracy |
| `find_by_namespace(namespace, collection_name, limit)` | Find metrics by Kubernetes namespace |
| `get_metrics_summary()` | Get summary statistics for stored metrics |
| `get_metrics_summary_async()` | Async version of get_metrics_summary |

#### Convenience Functions

| Function | Description |
|----------|-------------|
| `get_mongodb_client(config)` | Get a MongoDB client instance |
| `initialize_mongodb()` | Initialize MongoDB collections and indexes |

---

### utils/embedding.py

Text embedding utilities using Azure OpenAI.

| Class | Description |
|-------|-------------|
| `OpenAIEmbedding` | Interface for embedding text using Azure OpenAI SDK |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__(config)` | Initialize embedding client with Azure configuration |
| `embed_text(text)` | Embed a single text string (async) |
| `embed_batch(texts)` | Embed multiple text strings in batch (async) |
| `aembed_documents(texts)` | Async embedding for semantic cache |
| `aembed_query(text)` | Async embed query text for semantic cache |
| `embed_documents(texts)` | Sync embedding for semantic cache |
| `embed_query(text)` | Sync embed query text for semantic cache |
| `close()` | Close client connections |

---

### utils/file_storage.py

Azure Blob Storage client for file management.

| Class | Description |
|-------|-------------|
| `AsyncFileStorage` | Async class for Azure Blob Storage operations |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__(config)` | Initialize with storage configuration |
| `create_file_storage_connection()` | Create connection pool for each container |
| `read_file(container_name, file_name)` | Read file contents from a container |
| `upload_file(container_name, local_file_path, container_path)` | Upload file to blob storage |
| `list_files(container_name, regex_pattern)` | List files in container with optional regex filter |
| `delete_file(container_name, file_path)` | Delete file from container |
| `close()` | Close all blob storage client connections |

---

### utils/rai_util.py

Responsible AI content safety utilities using Azure Content Safety.

| Class | Description |
|-------|-------------|
| `RAIContentSafety` | Wrapper for Azure Content Safety client |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__(rai_config)` | Initialize with Content Safety endpoint and API key |
| `analyze_text(text)` | Analyze text for content safety violations |
| `close()` | Close the client session |

---

### utils/custom_errors.py

Custom exception classes for error handling.

| Class | Description |
|-------|-------------|
| `MyCustomError` | Base custom error class with traceback logging |
| `AsyncPostgresUtilError` | Error for AsyncPostgresUtil operations |
| `QuotaManagementError` | Error for quota management |
| `SessionManagementError` | Error for session management |
| `ChatHistoryError` | Error for chat history operations |
| `AuditLogError` | Error for audit logging |
| `AsyncFileStorageError` | Error for file storage operations |
| `OrchestratorError` | Error for orchestrator operations |
| `ResponsibleAIUtilError` | Error for RAI utilities |
| `SemanticRedisCacheError` | Error for semantic cache |
| `AzureOpenAIClientError` | Error for Azure OpenAI client |
| `LLMError` | Error for LLM operations |
| `PythonGenerationAgentError` | Error for Python generation agent |
| `RagAgentError` | Error for RAG agent |
| `PromptManagerError` | Error for prompt management |

---

### utils/setup_logging.py

Logging configuration utilities.

| Class | Description |
|-------|-------------|
| `SetupLogging` | Class to setup application logging |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__()` | Initialize and configure Azure logging |
| `get_logger(log_file, level)` | Static method to setup and return a logger with file and console handlers |

#### Functions

| Function | Description |
|----------|-------------|
| `configure_azure_logging()` | Configure Azure SDK logging to reduce verbosity |

---

## Data Module

### data/ai_search.py

Azure AI Search integration for semantic and hybrid search.

| Class | Description |
|-------|-------------|
| `AzureAISearch` | Async class for Azure AI Search operations |

#### Methods

| Method | Description |
|--------|-------------|
| `__init__(config, credential, embedding_client)` | Initialize AI Search client |
| `_get_index_client()` | Get or create async index client |
| `_get_search_client(index_name)` | Get or create async search client |
| `define_index_schema(vector_dimensions, ...)` | Define index schema with vector and semantic configuration |
| `create_index(index_schema, vector_dimensions, overwrite, index_name)` | Create search index |
| `ingest_chunks(chunks, batch_size, index_name)` | Ingest document chunks into the index |
| `semantic_search(query, query_vector, top_k, filters, ...)` | Perform semantic (vector) search |
| `hybrid_search(query, query_vector, top_k, filters, ...)` | Perform hybrid search (keyword + vector) |
| `keyword_search(query, top_k, filters, ...)` | Perform keyword-only search |
| `delete_chunks(filters, document_ids, index_name)` | Delete documents from index |

---

## Metrics Extraction Module (Detailed)

### notebooks/metrics_extraction/metrics_extractor.py

This is the core module for extracting metrics from IT-Ops agent run report files. It uses LLM-based analysis to extract both quantitative and qualitative metrics from agent logs.

#### Class: `MetricsExtractor`

Main class for extracting metrics from IT-Ops agent run reports.

##### Class Attributes

| Attribute | Description |
|-----------|-------------|
| `PATTERNS` | Dictionary of regex patterns for extracting various metrics (session_id, namespace, deployment, timestamps, tokens, etc.) |
| `QUANTITATIVE_PROMPT` | Detailed prompt template for LLM to extract quantitative metrics |
| `QUALITATIVE_PROMPT` | Detailed prompt template for LLM to extract qualitative metrics |

##### Instance Attributes

| Attribute | Description |
|-----------|-------------|
| `file_path` | Path to the agent run report file |
| `content` | Loaded file content |
| `errors` | List of errors encountered during extraction |
| `warnings` | List of warnings during extraction |

##### Constructor

```python
def __init__(self, file_path: str)
```
Initialize the extractor with a file path to the agent run report.

##### Methods

###### File Loading

| Method | Description |
|--------|-------------|
| `load_file()` | Load the file content from disk. Returns `True` on success, `False` on failure. |

###### Content Chunking

| Method | Description |
|--------|-------------|
| `_iter_content_chunks(chunk_size=80000)` | Split content into fixed-size chunks for processing large files. Returns list of overlapping chunks. |

###### Merging Utilities

| Method | Description |
|--------|-------------|
| `_merge_tool_calls(base_calls, new_calls)` | Static method to merge tool call lists while avoiding duplicates |
| `_merge_text(base, new)` | Static method to merge text fields by appending if different |
| `_merge_quantitative(base, new)` | Merge quantitative extraction results, preferring non-default values |
| `_merge_qualitative(base, new)` | Merge qualitative extraction results, preferring non-default values |

###### LLM-Based Extraction

| Method | Description |
|--------|-------------|
| `extract_quantitative_with_llm(llm_client)` | Use LLM to extract quantitative metrics. Processes content in chunks and merges results. Returns `LLMQuantitativeExtraction`. |
| `extract_qualitative_with_llm(llm_client)` | Use LLM to extract qualitative metrics. Processes content in chunks and merges results. Returns `LLMQualitativeExtraction`. |
| `extract_with_llm(llm_client)` | Main extraction method. Runs quantitative and qualitative extraction in parallel, saves to MongoDB, returns `MetricsExtractionResult`. |

###### MongoDB Persistence

| Method | Description |
|--------|-------------|
| `_save_llm_metrics_to_mongodb(quantitative, qualitative)` | Persist LLM-extracted metrics to MongoDB with session ID and metadata. |

#### Quantitative Metrics Extracted

The `extract_quantitative_with_llm` method extracts:

| Metric | Description |
|--------|-------------|
| `session_id` | Unique session identifier (UUID format) |
| `namespace` | Kubernetes namespace |
| `deployment_name` | Deployment/application name |
| `fault_injection_time` | Timestamp when fault was injected |
| `agent_fault_detection_time` | Timestamp when agent detected the fault |
| `agent_fault_mitigation_time` | Timestamp when agent mitigated the fault |
| `framework_overhead_seconds` | Framework overhead in seconds |
| `fault_detected` | Type of fault detected by the agent |
| `fault_type` | Type of fault injected (e.g., "Misconfig") |
| `fault_target_service` | Service where fault was injected |
| `fault_namespace` | Namespace of the faulty service |
| `trajectory_steps` | Number of steps in the agent trajectory |
| `input_tokens` | Total input tokens used |
| `output_tokens` | Total output tokens used |
| `tool_calls` | List of tool calls with name, arguments, success status |

#### Qualitative Metrics Extracted

The `extract_qualitative_with_llm` method extracts:

| Metric | Description |
|--------|-------------|
| `rai_check_status` | Responsible AI check status ('Passed', 'Failed', 'Not Evaluated') |
| `rai_check_notes` | Notes on RAI compliance |
| `trajectory_efficiency_score` | Efficiency score 0-10 |
| `trajectory_efficiency_notes` | Efficiency assessment details |
| `security_compliance_status` | Security compliance status |
| `security_compliance_notes` | Security compliance notes |
| `acceptance_criteria_met` | Boolean - was anomaly correctly detected? |
| `acceptance_criteria_notes` | Acceptance criteria evaluation |
| `response_quality_score` | Response quality score 0-10 |
| `response_quality_notes` | Response quality assessment |
| `reasoning_judgement` | Overall reasoning judgement |
| `reasoning_score` | Reasoning score 0-10 |
| `known_limitations` | List of observed limitations |
| `recommendations` | List of actionable improvements |
| `agent_summary` | Concise summary of agent's actions and findings |

#### Convenience Functions

| Function | Description |
|----------|-------------|
| `extract_metrics_from_file(file_path)` | Extract metrics from a single file using regex (fallback method) |
| `extract_metrics_from_multiple_files(file_paths)` | Extract metrics from multiple files using regex |
| `extract_metrics_with_llm(file_path, llm_client)` | **Recommended**: Extract metrics using LLM |
| `extract_metrics_from_multiple_files_with_llm(file_paths)` | Extract metrics from multiple files using LLM |

#### Usage Example

```python
import asyncio
from notebooks.metrics_extraction import extract_metrics_with_llm

async def main():
    # Extract metrics from a single file
    result = await extract_metrics_with_llm("agentcert/data/pid_run_1.txt")
    
    if result.success:
        metrics = result.metrics
        print(f"Session ID: {metrics['quantitative_metrics']['session_id']}")
        print(f"Trajectory Steps: {metrics['quantitative_metrics']['trajectory_steps']}")
        print(f"RAI Status: {metrics['qualitative_metrics']['rai_check_status']}")
        print(f"Recommendations: {metrics['qualitative_metrics']['recommendations']}")
    else:
        print(f"Errors: {result.errors}")

asyncio.run(main())
```

#### Processing Flow

1. **File Loading**: The `load_file()` method reads the agent run report file
2. **Chunking**: Large files are split into overlapping chunks (~80KB each)
3. **Parallel Extraction**: Quantitative and qualitative extraction run concurrently
4. **Chunk Processing**: Each chunk is sent to the LLM with appropriate prompts
5. **Result Merging**: Results from multiple chunks are merged intelligently
6. **MongoDB Persistence**: Final results are saved to MongoDB
7. **Return**: `MetricsExtractionResult` with success status, metrics, errors, and warnings

---

## Package Exports

### notebooks/metrics_extraction/__init__.py

Exports the following for external use:

```python
__all__ = [
    "MetricsExtractor",
    # LLM-based extraction (recommended)
    "extract_metrics_with_llm",
    "extract_metrics_from_multiple_files_with_llm",
    # Regex-based extraction (fallback)
    "extract_metrics_from_file",
    "extract_metrics_from_multiple_files",
    # LLM extraction models
    "LLMQuantitativeExtraction",
    "LLMQualitativeExtraction",
]
```

---

## Dependencies

Key dependencies from `requirements.txt`:

| Package | Version | Purpose |
|---------|---------|---------|
| `agent-framework` | 1.0.0b251223 | Azure OpenAI agent framework |
| `azure-ai-contentsafety` | 1.0.0 | Content safety analysis |
| `azure-identity` | 1.25.1 | Azure authentication |
| `azure-search-documents` | 11.7.0b2 | Azure AI Search |
| `azure-storage-blob` | 12.27.1 | Blob storage |
| `motor` | 3.7.1 | Async MongoDB driver |
| `openai` | 2.14.0 | OpenAI SDK |
| `pydantic` | 2.12.5 | Data validation |
| `pymongo[srv]` | 4.16.0 | MongoDB driver |

---

## Environment Variables

Required environment variables (see `.env.example`):

| Variable | Description |
|----------|-------------|
| `MONGODB_CONNECTION_STRING` | MongoDB Atlas connection string |
| `AZURE_OPENAI_ENDPOINT` | Azure OpenAI endpoint URL |
| `AZURE_OPENAI_API_KEY` | Azure OpenAI API key |
| `AZURE_OPENAI_DEPLOYMENT` | Azure OpenAI deployment name |
| `AZURE_EMBEDDING_ENDPOINT` | Azure embedding model endpoint |
| `AZURE_EMBEDDING_API_KEY` | Azure embedding API key |
| `AZURE_AI_SEARCH_ENDPOINT` | Azure AI Search endpoint |
| `AZURE_AI_SEARCH_API_KEY` | Azure AI Search API key |
| `AZURE_CONTENT_SAFETY_ENDPOINT` | Azure Content Safety endpoint |
| `AZURE_CONTENT_SAFETY_API_KEY` | Azure Content Safety API key |
| `AZURE_STORAGE_CONNECTION_STRING` | Azure Blob Storage connection string |

---

## License

This project is proprietary software developed for IT-Ops Agent certification purposes.
