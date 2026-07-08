# lumine
一个轻量级本地 HTTP/SOCKS5 代理服务器，保护基于 TCP 的 TLS 连接。

## 安装

```
go install github.com/moi-si/lumine@latest
```

## 编译

```
git clone https://github.com/moi-si/lumine
cd lumine
go build
```

## 文档

[https://github.com/moi-si/lumine/wiki/%E6%96%87%E6%A1%A3](https://github.com/moi-si/lumine/wiki/%E6%96%87%E6%A1%A3)

# 致谢

本项目中的技术最初源自 Python 工具 [TlsFragment](https://github.com/maoist2009/TlsFragment)。

我们用 Go 语言重写了整个实现，最终得到了一个速度更快、功能更丰富的版本，其配置文件与原版相似，但并不兼容。

# 开源许可

GPLv3