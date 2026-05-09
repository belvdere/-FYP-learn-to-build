# Data Model Documentation

## Overview

The backend stores a unified graph in `.fyp/index.db`:
- Real code entities from indexing (`symbols`, `method_calls`)
- Virtual planning entities (`virtual_nodes`, `virtual_directories`)
- Enrichment and workflow state (annotations, snapshots, masking sessions)

## Core Tables

### Real Code Index

#### `symbols`
- `id` (PK): canonical symbol ID (for example file-based method ID)
- `name`, `kind`, `file_path`, `line`, `signature`, `created_at`

#### `method_calls`
- `id` (PK autoincrement)
- `caller_id`, `callee_id` (canonical symbol IDs)
- `call_type`, `file_path`, `line`, `created_at`

Indexing writes to these tables via `ReplaceIndexData(...)` (initial batch) and `BulkInsertEdges(...)` (remaining edge batches).

### Virtual Graph Entities

#### `virtual_nodes`
- `id`, `label`, `type`
- `virtual_class_id`, `virtual_directory`
- `is_virtual`, `materialized`, timestamps

#### `virtual_directories`
- `id`, `path`, `name`, `parent_path`, `created_at`

### Enrichment

#### `node_annotations`
- `node_id`, `description`, `ai_remarks`, `code_snippet`, `custom_properties`, timestamps

#### `edge_annotations`
- `edge_id`, `source_node_id`, `target_node_id`, `remarks`, `call_count`, timestamps

### Workflow State

#### `snapshots`
- `id`, `name`, `virtual_nodes`, `context_nodes`, `edges`, `custom_prompt`, timestamps

#### `mask_sessions`
- `id`, `file_path`, `original_code`, `masked_code`, `filled_code`, `language`, `status`, timestamps

## Indexing Data Flow

1. Extension calls `index.scan` to collect symbols.
2. Extension resolves outgoing calls through VS Code call hierarchy.
3. Extension sends initial payload to `index.storeEdges` (symbols + first edge batch).
4. Extension streams remaining edge batches with `index.appendEdges`.
5. Backend persists canonical symbols/calls transactionally.

On-save incremental flow:
1. `index.scanFile` computes changed/removed callers using method body hashes.
2. Extension resolves outgoing calls for changed callers.
3. `index.storeFileDelta` updates only affected rows in `symbols`/`method_calls`.

`index.run` remains a deprecated compatibility stub and does not index.

## Query Semantics

- Graph APIs merge real symbols and virtual nodes into one node set.
- Real edges come from `method_calls`.
- Manual/annotated relationships come from `edge_annotations`.
- Neighborhood queries check both symbol labels and canonical IDs.

## Indexes

Critical indexes include:
- `idx_symbols_name`, `idx_symbols_file`
- `idx_method_calls_caller` on `caller_id`
- `idx_method_calls_callee` on `callee_id`
- `idx_method_calls_file`
- annotation and virtual hierarchy indexes

These support graph loading, search, and neighborhood expansion in the webview.
