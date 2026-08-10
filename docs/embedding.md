# Embedding & sqlite-vector (S2)

## Behavior

| 状态 | 搜索 |
| --- | --- |
| 未配置向量模型 | **仅 FTS5 全文** |
| 已配置模型，扩展未加载 | FTS + **进程内余弦**（小库可用） |
| 已配置 + `vector.dll` 加载成功 | FTS + **sqlite-vector**（quantize_scan） |

## 配置项（`settings` 表）

- `embedding.provider`: `disabled` | `openai_compatible`
- `embedding.base_url`, `embedding.api_key`, `embedding.model`
- `embedding.dimensions`, `embedding.batch_size`
- `search.mode`: `auto` | `fts` | `vector` | `hybrid`

判定「已配置」：`provider=openai_compatible` 且 model、dimensions、base_url 有效。

## 扩展路径

1. 环境变量 `LRSS_VECTOR_LIB`
2. 可执行文件旁 `lib/vector.dll`（Windows）
3. 仓库 `third_party/sqlite-vector/windows_amd64/vector.dll`

当前已放入 Windows x64 1.0.0 预编译库。

### modernc.org/sqlite 说明

纯 Go 驱动对动态 `load_extension` **支持有限**。启动时会尝试加载；失败不致命，日志打印原因。  
进程内 brute-force 余弦保证「已配置时」向量检索仍可用。

若需强制原生 sqlite-vector：后续可切换 `mattn/go-sqlite3` + CGO。

## API（Wails）

- `SettingsService.GetEmbeddingConfig` / `SetEmbeddingConfig`
- `SettingsService.GetSearchCapabilities` / `GetVectorStatus`
- `SettingsService.RunEmbedOnce(limit)`
- `SearchService.Search(query, mode, limit)`

## 开发

```bash
go test ./internal/...
```
