# passauto

自动应答交互式密码提示（密码、PIN、口令）的命令行工具。类似 `expect`，但更简单，且内置加密密钥管理。

[English](README.md)

## 为什么选 passauto？

| 工具 | 脚本 | 密钥存储 | 跨平台 | 学习成本 |
|------|------|----------|--------|----------|
| **passauto** | 无需脚本，一行命令 | AES-256-GCM，从 SSH key 派生 | Windows + Linux + macOS | 极低 |
| expect/pexpect | TCL/Python 脚本 | 无（明文写在脚本里） | 仅 Linux/macOS | 中等 |
| sshpass | 单条命令 | 明文参数或环境变量 | 仅 Linux | 低 |
| ansible vault | Playbook 级别 | 加密 vault | 通过 Ansible | 高 |

## 安装

### 下载二进制

从 [Releases](https://github.com/lifefinity/passauto/releases/latest) 下载：

```bash
# Linux (amd64)
curl -sL https://github.com/lifefinity/passauto/releases/latest/download/passauto-linux-amd64 -o passauto
chmod +x passauto && sudo mv passauto /usr/local/bin/

# macOS (Apple Silicon)
curl -sL https://github.com/lifefinity/passauto/releases/latest/download/passauto-darwin-arm64 -o passauto
chmod +x passauto && sudo mv passauto /usr/local/bin/

# Windows — 从 Releases 页面下载 passauto-windows-amd64.exe
```

### 从源码编译

```bash
git clone https://github.com/lifefinity/passauto.git
cd passauto && make build
```

## 快速开始

```bash
# 编译
make build    # → bin/passauto.exe

# 1. 添加 profile
passauto add -c "ssh user@server" -m "password:" -d "生产服务器" myserver

# 2. 运行 — 密码自动填充
passauto myserver

# 3. 查看已有配置
passauto list
```

## 工作原理

```
passauto myserver
    │
    ├─ 从 ~/.passauto/data.json 加载 profile
    ├─ 从 SSH 私钥派生 AES 密钥 (HKDF-SHA256)
    ├─ 解密存储的 secret
    ├─ 在伪终端中启动命令
    ├─ 监听输出 → 正则匹配 → 自动输入 secret
    └─ 进程正常退出
```

### KMS 模式（团队/企业）

当 profile 设置了 `--kms-key-id` 时，passauto 使用 AWS KMS 信封加密代替 SSH 密钥派生：

```
passauto myserver
    ├─ 调用 KMS GenerateDataKey -> 获取明文 DEK + 加密 DEK
    ├─ 使用 DEK 加密 secret (AES-256-GCM)
    └─ 存储加密 DEK + 密文
```

## 示例

### 常用 Profile

```bash
# SSH 服务器
passauto add -c "ssh deploy@prod-server" -m "password:" -d "生产部署" prod

# PostgreSQL
passauto add -c "psql -h db.example.com -U admin mydb" -m "password" -p "=>\s*$" -d "主数据库" mydb

# MySQL
passauto add -c "mysql -h db.example.com -u root -p" -m "password:" -d "MySQL 生产" mysql-prod

# Sudo
passauto add -c "sudo apt upgrade -y" -m "password" -d "系统更新" apt-upgrade

# Kerberos
passauto add -c "kinit admin@EXAMPLE.COM" -m "password for" -d "Kerberos 认证" krb

# Midway (Amazon)
passauto add -c "kinit admin@CORP.COM" -m "Password:" --after "klist" -d "Kerberos 认证" krb
```

### 登录后执行命令 (--then)

在密码自动填充后，在会话内执行命令（需要 `-p` 指定 shell 提示符）：

```bash
# 连接后执行 SQL
passauto mydb --then "SELECT now();" --then "\q"

# 从文件执行
passauto mydb --script queries.sql

# 组合使用
passauto mydb --then "\timing on" --script queries.sql --then "\q"
```

### 退出后执行命令 (--after)

主进程正常退出后，在新 shell 中执行命令：

```bash
# kinit 完成后显示凭据
passauto krb --after "klist"

# SSH 退出后同步文件
passauto prod --after "rsync -a ./dist/ server:/app/"
```

### 注入环境变量 (--env / -e)

```bash
passauto deploy -e HOST=prod.example.com -e PORT=5432
```

### 更新 Profile

```bash
passauto update prod --secret                    # 更换密码
passauto update prod -c "ssh newuser@host"       # 更换命令
passauto update prod -d "新的描述"                # 更换描述
passauto update mydb --then "\timing on"         # 设置登录后步骤
passauto update krb --after "klist"            # 设置退出后命令
passauto update mysql-prod -m "password:" -t 60s # 更换匹配和超时
```

### 静默模式

```bash
passauto mydb --quiet --script queries.sql    # 无终端输出
passauto mydb -q --then "SELECT 1;"           # 短写
```

## 命令一览

| 命令 | 说明 |
|------|------|
| `passauto <profile> [-s service]` | 运行 profile，自动应答 |
| `passauto add <profile>` | 创建新 profile |
| `passauto update <profile>` | 更新 profile 字段 |
| `passauto list` | 列出所有 profile |
| `passauto remove <profile>` | 删除 profile |
| `passauto change-key <path>` | 更换加密 SSH key |
| `passauto export <file>` | 导出 profile（不含密码） |
| `passauto import <file>` | 导入 profile |
| `passauto backup <dir>` | 备份密钥和数据 |
| `passauto restore <dir>` | 恢复密钥和数据 |
| `passauto completion <shell>` | 生成 shell 补全脚本 |
| `passauto version` | 版本信息 |
| `passauto init` | 首次初始化 |

## --then 与 --after 的区别

| | `--then` | `--after` |
|---|----------|-----------|
| **何时执行** | 在运行的会话内部 | 主进程退出后 |
| **需要** | `-p` 指定 shell 提示符 | 无要求 |
| **适用场景** | psql、mysql、ssh 交互式 shell | kinit、docker login 等一次性命令 |
| **执行环境** | PTY 会话内 | 新的 `sh -c` shell |

## 模式匹配

- **默认大小写不敏感** — `"password"` 匹配 `Password for user demo1:`、`PASSWORD:` 等
- 使用 `--case-sensitive` 启用精确匹配
- 模式是 Go 正则表达式（如 `"password|passphrase"` 匹配两者之一）
- 部分匹配即可，不需要完整行匹配

## 更换加密密钥

```bash
passauto change-key ~/.ssh/id_ed25519_new
```

用旧 key 解密所有 secret，用新 key 重新加密。两个 key 都可以有密码保护。

## 多服务 Profile

当一台服务器有多个服务（SSH、PostgreSQL、Oracle 等）时，使用 `-s` 区分：

```bash
# 为同一服务器添加多个服务
passauto add -c "ssh admin@prod" -m "password:" prod -s ssh
passauto add -c "psql -h prod -U admin" -m "password" prod -s pg
passauto add -c "sqlplus admin@prod-orcl" -m "password:" prod -s oracle

# 运行 -- 名称唯一时直接运行
passauto prod              # 多个匹配 -> 显示选择菜单
passauto prod -s ssh       # 精确匹配 -> 直接运行

# list 显示 service 列
passauto list
# NAME   SERVICE   COMMAND                          DESCRIPTION
# prod   ssh       ssh admin@prod                   ...
# prod   pg        psql -h prod -U admin            ...
# prod   oracle    sqlplus admin@prod-orcl           ...
```

唯一性约束基于 `(name, service)` 对。不带 `-s` 的 profile service 字段为空。

## 钥匙串缓存

passauto 将派生的 AES 加密密钥缓存在操作系统钥匙串中（macOS Keychain、Linux secret-service、Windows Credential Manager），避免每次运行都读取 SSH 密钥。

- 缓存 TTL：1 小时（自动过期）
- 按 profile 隔离（每个 profile 独立缓存）
- 使用 `--no-cache` 禁用

```bash
passauto prod              # 首次运行：读取 SSH key，缓存派生密钥
passauto prod              # 后续运行：使用缓存密钥（更快）
passauto prod --no-cache   # 跳过缓存，重新从 SSH key 派生
```

## KMS 信封加密

用于团队/企业场景，passauto 支持 AWS KMS 信封加密。不再从本地 SSH 密钥派生，而是由 KMS 生成和管理数据加密密钥。

```bash
# 使用 KMS 加密添加 profile
passauto add -c "ssh admin@prod" -m "password:" prod --kms-key-id arn:aws:kms:us-east-1:123456:key/abc-def

# 已有 profile：切换到 KMS
passauto update prod --kms-key-id arn:aws:kms:us-east-1:123456:key/abc-def

# 正常运行 -- KMS 解密透明进行
passauto prod
```

要求：
- 已配置 AWS 凭证（`~/.aws/credentials` 或环境变量）
- IAM 权限：对指定密钥的 `kms:GenerateDataKey`、`kms:Decrypt`

## 备份与恢复

```bash
passauto backup /mnt/usb/passauto-backup     # 备份
passauto restore /mnt/usb/passauto-backup    # 恢复
passauto restore ~/backup --force            # 覆盖现有数据
```

导出/导入（不含密钥，适合共享配置）：

```bash
passauto export profiles.json                # 导出（不含 secret）
passauto import profiles.json                # 导入合并
passauto import profiles.json --force        # 覆盖同名
```

## Shell 补全

```bash
# Bash — 加入 ~/.bashrc
eval "$(passauto completion bash)"

# Zsh — 加入 ~/.zshrc
eval "$(passauto completion zsh)"

# Fish
passauto completion fish > ~/.config/fish/completions/passauto.fish
```

支持 Tab 补全 profile 名称。

## 安全性

- 使用 **AES-256-GCM** 加密（每个 secret 独立随机 nonce）
- 加密密钥从 **SSH 私钥** 通过 HKDF-SHA256 派生，不存储在磁盘上
- 如果没有 SSH key，自动生成 `~/.passauto/passauto_key`（ed25519）
- 数据文件 `~/.passauto/data.json` 权限为 0600
- 任何地方都没有明文 secret

## 平台支持

| 平台 | 方式 |
|------|------|
| Windows 10+ | ConPTY |
| Linux | PTY (creack/pty) |
| macOS | PTY (creack/pty) |

## 文档

- [用户指南](docs/user-guide.md) — 详细用法、标志、故障排除
- [架构设计](docs/architecture.md) — 组件设计、数据流、安全模型
- [开发指南](docs/development.md) — 编译、测试、贡献
