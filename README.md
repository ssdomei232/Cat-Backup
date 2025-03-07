# Cat-Backup

*A reliable backup solution that lands on its feet, just like a cat!*

Cat-Backup 是一个基于 Go 语言开发的自动化备份工具，专为需要将数据备份到远程 WebDAV 服务器的场景设计。它提供了灵活的配置、强大的日志记录和错误处理机制，确保您的数据安全无忧。

---

## 功能特性

* 🕒 **定时备份**：支持 Cron 表达式，灵活配置备份频率
* ☁️ **远程存储**：将备份文件上传至 WebDAV 服务器
* 📦 **压缩打包**：自动将备份目录压缩为 `.tar.gz` 文件
* 📝 **详细日志**：记录每个备份任务的执行过程和结果
* 📧 **邮件通知**：备份失败时发送邮件提醒
* 🔄 **失败重试**：自动重试失败的上传操作
* ✅ **配置验证**：启动时自动检查配置文件的正确性
* 🗂️ **缓存管理**：自动清理临时压缩文件

---

## 快速开始

### 1. 安装依赖

**(当然，您也可以尝试在发行版处寻找合适的版本，那样您就可以跳过第一步和第三步)**
确保已安装 Go 1.16+，然后运行以下命令安装依赖：

```bash
go mod tidy
```

### 2. 配置文件

创建一个 `.env` 文件，内容示例如下：

```ini
# WebDAV 配置
WEBDAV_URL=https://your-webdav-server.com/dav
WEBDAV_USER=backup_user
WEBDAV_PASSWORD=secure_password

# 缓存目录配置
CACHE_DIR=/var/backup_cache

# 备份任务数量
BACKUP_COUNT=2

# 第一个备份任务
BACKUP_1_NAME=app_data
BACKUP_1_SOURCE=/var/www/app/data
BACKUP_1_REMOTE_PATH=/backups/app
BACKUP_1_FREQUENCY=@daily

# 第二个备份任务
BACKUP_2_NAME=db_backup
BACKUP_2_SOURCE=/opt/mysql/backups
BACKUP_2_REMOTE_PATH=/backups/database
BACKUP_2_FREQUENCY=0 2 * * *

# SMTP 配置
SMTP_HOST=smtp.example.com
SMTP_USER=alerts@example.com
SMTP_PASSWORD=email_password
SMTP_TO=admin1@example.com,admin2@example.com
```

### 3. 编译

编译 Cat-Backup：

```bash
go build -o cat-backup
```

### 4. 运行

```bash
go build -o cat-backup 
```

---

## 配置说明

### 必需配置

|配置项|说明|
| --- | --- |
|`WEBDAV_URL`|WebDAV 服务器地址|
|`WEBDAV_USER`|WebDAV 用户名|
|`WEBDAV_PASSWORD`|WebDAV 密码|
|`CACHE_DIR`|临时文件缓存目录|
|`BACKUP_COUNT`|备份任务数量|

### 备份任务配置

每个备份任务以 `BACKUP_N_` 为前缀，其中 `N` 为任务序号（从 1 开始）。

|配置项|说明|
| --- | --- |
|`NAME`|备份任务名称|
|`SOURCE`|需要备份的本地目录|
|`REMOTE_PATH`|WebDAV 服务器上的存储路径|
|`FREQUENCY`|备份频率（Cron 表达式）|

### 可选配置

|配置项|说明|
| --- | --- |
|`SMTP_HOST`|SMTP 服务器地址|
|`SMTP_USER`|SMTP 用户名|
|`SMTP_PASSWORD`|SMTP 密码|
|`SMTP_TO`|接收通知的邮箱地址（逗号分隔）|

---

## 日志示例

```plaintext
2023/10/15 14:30:01 [INFO] Starting backup service... 
2023/10/15 14:30:01 [INFO] Scheduling backup 'app_data' with cron: @daily 
2023/10/15 14:30:01 [INFO] Service started successfully 
2023/10/15 00:00:00 [INFO] [app_data] Backup started 
2023/10/15 00:00:00 [INFO] [app_data] Creating archive... 
2023/10/15 00:02:15 [INFO] [app_data] Created archive (145.32 MB) 
2023/10/15 00:02:15 [INFO] [app_data] Uploading (attempt 1/3)... 
2023/10/15 00:02:17 [WARN] [app_data] Upload failed: 404 Not Found (retrying in 1s) 
2023/10/15 00:02:18 [INFO] [app_data] Uploading (attempt 2/3)... 
2023/10/15 00:02:19 [INFO] [app_data] Upload successful 2023/10/15 00:02:19 [INFO] [app_data] Backup completed in 2m19s
```

---

## 故障排查

1. **备份失败**：

* 检查日志中的错误信息
* 确保 WebDAV 服务器可访问
* 验证本地目录权限

2. **邮件未发送**：

* 检查 SMTP 配置是否正确
* 确保网络连接正常
* 查看垃圾邮件文件夹

3. **性能问题**：

* 确保缓存目录有足够空间
* 调整备份频率
* 压缩大文件时增加系统资源

---

*Cat-Backup - Because your data deserves nine lives!* 🐾
