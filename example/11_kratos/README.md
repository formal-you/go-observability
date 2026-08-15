# 11 · kratos 传输层适配

目标：演示 go-kratos HTTP ErrorEncoder、错误日志中间件与 gRPC ErrorMapper。

从仓库根目录运行：

```powershell
go run ./example/11_kratos
Get-Content .\logs\kratos.jsonl
```

```bash
go run ./example/11_kratos
cat ./logs/kratos.jsonl
```

程序直接调用三个公开适配函数，不启动完整 kratos Server：

- `ErrorEncoder`：把 `errs.AppError` 映射为 kratos 错误契约；
- `ErrorLog`：handler 返回错误时写出 ErrorEvent；
- `GRPCErrorMapper`：把错误转成 gRPC status，避免内部 message 透传。
