#!/bin/bash

# 生产级 Python FaaS 运维管理脚本
# 提供完整的生产环境管理功能

set -euo pipefail

# 脚本配置
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.yml"
ENV_FILE="$PROJECT_ROOT/.env.production"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_debug() {
    echo -e "${BLUE}[DEBUG]${NC} $1"
}

# 检查依赖
check_dependencies() {
    local deps=("docker" "docker-compose" "curl" "jq")
    for dep in "${deps[@]}"; do
        if ! command -v "$dep" &> /dev/null; then
            log_error "依赖 $dep 未安装"
            exit 1
        fi
    done
}

# 检查环境文件
check_env_file() {
    if [[ ! -f "$ENV_FILE" ]]; then
        log_error "生产环境配置文件不存在: $ENV_FILE"
        log_info "请先创建生产环境配置文件"
        exit 1
    fi
}

# 加载环境变量
load_env() {
    if [[ -f "$ENV_FILE" ]]; then
        set -a
        source "$ENV_FILE"
        set +a
        log_info "已加载生产环境配置"
    fi
}

# Docker Compose 命令包装
dc() {
    docker-compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" "$@"
}

# 服务健康检查
health_check() {
    local service="$1"
    local url="$2"
    local max_attempts=30
    local attempt=1
    
    log_info "检查服务健康状态: $service"
    
    while [[ $attempt -le $max_attempts ]]; do
        if curl -s -f "$url" > /dev/null 2>&1; then
            log_info "服务 $service 健康检查通过"
            return 0
        fi
        
        log_debug "健康检查尝试 $attempt/$max_attempts 失败，等待重试..."
        sleep 2
        ((attempt++))
    done
    
    log_error "服务 $service 健康检查失败"
    return 1
}

# 等待服务就绪
wait_for_service() {
    local service="$1"
    local port="$2"
    local timeout="${3:-60}"
    
    log_info "等待服务 $service 在端口 $port 就绪..."
    
    local start_time=$(date +%s)
    while true; do
        if nc -z localhost "$port" 2>/dev/null; then
            log_info "服务 $service 已就绪"
            return 0
        fi
        
        local current_time=$(date +%s)
        local elapsed=$((current_time - start_time))
        
        if [[ $elapsed -ge $timeout ]]; then
            log_error "等待服务 $service 超时"
            return 1
        fi
        
        sleep 2
    done
}

# 启动生产环境
start_production() {
    log_info "启动生产环境..."
    
    # 创建必要的目录
    create_data_directories
    
    # 启动基础设施服务
    log_info "启动基础设施服务..."
    dc up -d redis mysql clickhouse minio rocketmq-namesrv rocketmq-broker
    
    # 等待基础设施服务就绪
    wait_for_service "redis" "${COZE_LOOP_REDIS_PORT:-6379}"
    wait_for_service "mysql" "${COZE_LOOP_MYSQL_PORT:-3306}"
    wait_for_service "clickhouse" "${COZE_LOOP_CLICKHOUSE_PORT:-8123}"
    wait_for_service "minio" "${COZE_LOOP_OSS_PORT:-9000}"
    
    # 运行初始化服务
    log_info "运行数据库初始化..."
    dc up mysql-init clickhouse-init minio-init rocketmq-init
    
    # 启动 FaaS 服务
    log_info "启动 FaaS 服务..."
    dc up -d coze-loop-python-faas coze-loop-js-faas
    
    # 等待 FaaS 服务就绪
    wait_for_service "python-faas" "${COZE_LOOP_PYTHON_FAAS_PORT:-8890}"
    wait_for_service "js-faas" "${COZE_LOOP_JS_FAAS_PORT:-8891}"
    
    # 启动应用服务
    log_info "启动应用服务..."
    dc up -d app
    
    # 等待应用服务就绪
    wait_for_service "app" "${COZE_LOOP_APP_OPENAPI_PORT:-8888}"
    
    # 启动 Nginx
    log_info "启动 Nginx..."
    dc up -d nginx
    
    # 等待 Nginx 就绪
    wait_for_service "nginx" "${COZE_LOOP_NGINX_PORT:-80}"
    
    # 启动监控服务（如果启用）
    if [[ "${ENABLE_MONITORING:-false}" == "true" ]]; then
        log_info "启动监控服务..."
        dc --profile monitoring up -d prometheus grafana loki
        
        wait_for_service "prometheus" "${PROMETHEUS_PORT:-9090}"
        wait_for_service "grafana" "${GRAFANA_PORT:-3000}"
        wait_for_service "loki" "${LOKI_PORT:-3100}"
    fi
    
    log_info "生产环境启动完成！"
    show_service_status
}

# 停止生产环境
stop_production() {
    log_info "停止生产环境..."
    
    # 优雅停止应用服务
    log_info "停止应用服务..."
    dc stop app nginx
    
    # 停止 FaaS 服务
    log_info "停止 FaaS 服务..."
    dc stop coze-loop-python-faas coze-loop-js-faas
    
    # 停止监控服务
    if dc ps prometheus &>/dev/null; then
        log_info "停止监控服务..."
        dc stop prometheus grafana loki
    fi
    
    # 停止基础设施服务
    log_info "停止基础设施服务..."
    dc stop redis mysql clickhouse minio rocketmq-namesrv rocketmq-broker
    
    log_info "生产环境已停止"
}

# 重启生产环境
restart_production() {
    log_info "重启生产环境..."
    stop_production
    sleep 5
    start_production
}

# 创建数据目录
create_data_directories() {
    local data_dir="${DATA_DIR:-./data}"
    
    log_info "创建数据目录: $data_dir"
    
    mkdir -p "$data_dir"/{redis,mysql,clickhouse,minio,minio-config,rmq-namesrv,rmq-broker,prometheus,grafana,loki}
    
    # 设置权限
    chmod 755 "$data_dir"
    chmod 777 "$data_dir"/redis
    chmod 999 "$data_dir"/mysql
    chmod 101:101 "$data_dir"/clickhouse 2>/dev/null || true
    chmod 472:472 "$data_dir"/grafana 2>/dev/null || true
    chmod 10001:10001 "$data_dir"/loki 2>/dev/null || true
    
    log_info "数据目录创建完成"
}

# 显示服务状态
show_service_status() {
    log_info "服务状态概览:"
    echo
    
    # 检查容器状态
    echo -e "${CYAN}容器状态:${NC}"
    dc ps
    echo
    
    # 检查服务健康状态
    echo -e "${CYAN}服务健康检查:${NC}"
    
    local services=(
        "Python FaaS|http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/health"
        "JavaScript FaaS|http://localhost:${COZE_LOOP_JS_FAAS_PORT:-8891}/health"
        "应用服务|http://localhost:${COZE_LOOP_APP_OPENAPI_PORT:-8888}/health"
        "Nginx|http://localhost:${COZE_LOOP_NGINX_PORT:-80}"
    )
    
    if [[ "${ENABLE_MONITORING:-false}" == "true" ]]; then
        services+=(
            "Prometheus|http://localhost:${PROMETHEUS_PORT:-9090}/-/healthy"
            "Grafana|http://localhost:${GRAFANA_PORT:-3000}/api/health"
        )
    fi
    
    for service_info in "${services[@]}"; do
        IFS='|' read -r name url <<< "$service_info"
        if curl -s -f "$url" > /dev/null 2>&1; then
            echo -e "  ${GREEN}✅${NC} $name: 健康"
        else
            echo -e "  ${RED}❌${NC} $name: 不健康"
        fi
    done
    echo
}

# 查看日志
show_logs() {
    local service="${1:-}"
    local lines="${2:-100}"
    
    if [[ -z "$service" ]]; then
        log_info "显示所有服务日志 (最近 $lines 行):"
        dc logs --tail="$lines" -f
    else
        log_info "显示服务 $service 日志 (最近 $lines 行):"
        dc logs --tail="$lines" -f "$service"
    fi
}

# 执行基础性能测试
run_performance_test() {
    local test_type="${1:-basic}"
    local test_case="${2:-simple}"
    
    log_info "执行基础性能测试: $test_type"
    
    # 检查 Python FaaS 服务是否运行
    if ! health_check "Python FaaS" "http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/health"; then
        log_error "Python FaaS 服务未运行，无法执行性能测试"
        return 1
    fi
    
    # 执行基础性能测试
    log_info "测试 Python 代码执行..."
    
    # 简单计算测试
    local test_result=$(curl -s -X POST "http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/run_code" \
        -H "Content-Type: application/json" \
        -d '{"language":"python","code":"result = 1 + 2\nprint(f\"Result: {result}\")\nreturn_val(result)"}' \
        --max-time 10)
    
    if echo "$test_result" | grep -q '"status":"success"'; then
        log_info "✅ 基础计算测试通过"
    else
        log_error "❌ 基础计算测试失败: $test_result"
        return 1
    fi
    
    log_info "基础性能测试完成"
}

# 备份数据
backup_data() {
    local backup_dir="${BACKUP_STORAGE_PATH:-./backups}/$(date +%Y%m%d_%H%M%S)"
    local data_dir="${DATA_DIR:-./data}"
    
    log_info "开始数据备份到: $backup_dir"
    
    mkdir -p "$backup_dir"
    
    # 备份数据库
    log_info "备份 MySQL 数据..."
    dc exec mysql mysqldump -u"${COZE_LOOP_MYSQL_USER}" -p"${COZE_LOOP_MYSQL_PASSWORD}" "${COZE_LOOP_MYSQL_DATABASE}" > "$backup_dir/mysql_backup.sql"
    
    # 备份 ClickHouse 数据
    log_info "备份 ClickHouse 数据..."
    dc exec clickhouse clickhouse-client --query "BACKUP DATABASE ${COZE_LOOP_CLICKHOUSE_DATABASE} TO Disk('default', '$backup_dir/clickhouse_backup')" || true
    
    # 备份 Redis 数据
    log_info "备份 Redis 数据..."
    cp -r "$data_dir/redis" "$backup_dir/" 2>/dev/null || true
    
    # 备份 MinIO 数据
    log_info "备份 MinIO 数据..."
    cp -r "$data_dir/minio" "$backup_dir/" 2>/dev/null || true
    
    # 备份配置文件
    log_info "备份配置文件..."
    cp "$ENV_FILE" "$backup_dir/"
    cp -r "$PROJECT_ROOT/conf" "$backup_dir/" 2>/dev/null || true
    
    # 创建备份信息文件
    cat > "$backup_dir/backup_info.txt" << EOF
备份时间: $(date)
环境: production
版本: ${DEPLOYMENT_VERSION:-unknown}
数据目录: $data_dir
备份目录: $backup_dir
EOF
    
    log_info "数据备份完成: $backup_dir"
}

# 恢复数据
restore_data() {
    local backup_dir="$1"
    
    if [[ ! -d "$backup_dir" ]]; then
        log_error "备份目录不存在: $backup_dir"
        return 1
    fi
    
    log_warn "警告: 数据恢复将覆盖现有数据！"
    read -p "确认继续? (y/N): " -n 1 -r
    echo
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "数据恢复已取消"
        return 0
    fi
    
    log_info "开始数据恢复从: $backup_dir"
    
    # 停止服务
    stop_production
    
    # 恢复 MySQL 数据
    if [[ -f "$backup_dir/mysql_backup.sql" ]]; then
        log_info "恢复 MySQL 数据..."
        dc up -d mysql
        wait_for_service "mysql" "${COZE_LOOP_MYSQL_PORT:-3306}"
        dc exec -T mysql mysql -u"${COZE_LOOP_MYSQL_USER}" -p"${COZE_LOOP_MYSQL_PASSWORD}" "${COZE_LOOP_MYSQL_DATABASE}" < "$backup_dir/mysql_backup.sql"
    fi
    
    # 恢复 Redis 数据
    if [[ -d "$backup_dir/redis" ]]; then
        log_info "恢复 Redis 数据..."
        cp -r "$backup_dir/redis/"* "${DATA_DIR:-./data}/redis/" 2>/dev/null || true
    fi
    
    # 恢复 MinIO 数据
    if [[ -d "$backup_dir/minio" ]]; then
        log_info "恢复 MinIO 数据..."
        cp -r "$backup_dir/minio/"* "${DATA_DIR:-./data}/minio/" 2>/dev/null || true
    fi
    
    # 恢复配置文件
    if [[ -f "$backup_dir/.env.production" ]]; then
        log_info "恢复配置文件..."
        cp "$backup_dir/.env.production" "$ENV_FILE"
    fi
    
    log_info "数据恢复完成，重新启动服务..."
    start_production
}

# 清理旧备份
cleanup_old_backups() {
    local backup_dir="${BACKUP_STORAGE_PATH:-./backups}"
    local retention_days="${BACKUP_RETENTION_DAYS:-7}"
    
    log_info "清理 $retention_days 天前的备份..."
    
    find "$backup_dir" -type d -name "20*" -mtime +$retention_days -exec rm -rf {} \; 2>/dev/null || true
    
    log_info "备份清理完成"
}

# 监控系统资源
monitor_resources() {
    local duration="${1:-60}"
    local interval="${2:-5}"
    
    log_info "监控系统资源 $duration 秒 (每 $interval 秒采样一次)..."
    
    local end_time=$(($(date +%s) + duration))
    
    echo -e "${CYAN}时间\t\tCPU%\t内存%\t磁盘%\t网络(MB/s)${NC}"
    echo "----------------------------------------"
    
    while [[ $(date +%s) -lt $end_time ]]; do
        local timestamp=$(date +"%H:%M:%S")
        local cpu=$(top -bn1 | grep "Cpu(s)" | awk '{print $2}' | cut -d'%' -f1)
        local memory=$(free | grep Mem | awk '{printf "%.1f", $3/$2 * 100.0}')
        local disk=$(df / | tail -1 | awk '{print $5}' | cut -d'%' -f1)
        local network=$(cat /proc/net/dev | grep eth0 | awk '{print ($2+$10)/1024/1024}' 2>/dev/null || echo "0")
        
        printf "%s\t%.1f\t%.1f\t%s\t%.2f\n" "$timestamp" "$cpu" "$memory" "$disk" "$network"
        
        sleep "$interval"
    done
}

# 生成系统报告
generate_system_report() {
    local report_file="system_report_$(date +%Y%m%d_%H%M%S).txt"
    
    log_info "生成系统报告: $report_file"
    
    {
        echo "# 系统报告"
        echo "生成时间: $(date)"
        echo "环境: production"
        echo
        
        echo "## 系统信息"
        uname -a
        echo
        
        echo "## 内存使用"
        free -h
        echo
        
        echo "## 磁盘使用"
        df -h
        echo
        
        echo "## CPU 信息"
        lscpu | head -20
        echo
        
        echo "## 网络连接"
        netstat -tuln | head -20
        echo
        
        echo "## Docker 信息"
        docker version
        echo
        docker system df
        echo
        
        echo "## 容器状态"
        dc ps
        echo
        
        echo "## 服务健康检查"
        curl -s "http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/health" | jq . 2>/dev/null || echo "Python FaaS 不可用"
        echo
        curl -s "http://localhost:${COZE_LOOP_JS_FAAS_PORT:-8891}/health" | jq . 2>/dev/null || echo "JavaScript FaaS 不可用"
        echo
        
        if [[ "${ENABLE_MONITORING:-false}" == "true" ]]; then
            echo "## Prometheus 指标"
            curl -s "http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/metrics" | head -50
            echo
        fi
        
    } > "$report_file"
    
    log_info "系统报告已生成: $report_file"
}

# 更新服务
update_service() {
    local service="$1"
    
    log_info "更新服务: $service"
    
    # 拉取最新镜像
    dc pull "$service"
    
    # 重新创建容器
    dc up -d "$service"
    
    # 等待服务就绪
    case "$service" in
        "coze-loop-python-faas")
            wait_for_service "python-faas" "${COZE_LOOP_PYTHON_FAAS_PORT:-8890}"
            health_check "Python FaaS" "http://localhost:${COZE_LOOP_PYTHON_FAAS_PORT:-8890}/health"
            ;;
        "coze-loop-js-faas")
            wait_for_service "js-faas" "${COZE_LOOP_JS_FAAS_PORT:-8891}"
            health_check "JavaScript FaaS" "http://localhost:${COZE_LOOP_JS_FAAS_PORT:-8891}/health"
            ;;
        "app")
            wait_for_service "app" "${COZE_LOOP_APP_OPENAPI_PORT:-8888}"
            ;;
    esac
    
    log_info "服务 $service 更新完成"
}

# 扩缩容
scale_service() {
    local service="$1"
    local replicas="$2"
    
    log_info "扩缩容服务 $service 到 $replicas 个副本"
    
    dc up -d --scale "$service=$replicas" "$service"
    
    log_info "服务 $service 扩缩容完成"
}

# 显示帮助信息
show_help() {
    cat << EOF
生产级 Python FaaS 运维管理脚本

用法: $0 <命令> [参数]

命令:
  start                    启动生产环境
  stop                     停止生产环境
  restart                  重启生产环境
  status                   显示服务状态
  logs [service] [lines]   查看日志 (默认所有服务，100行)
  test [type] [case]       执行性能测试 (type: concurrent|load|stability|all, case: simple|math|numpy|complex)
  backup                   备份数据
  restore <backup_dir>     恢复数据
  cleanup-backups          清理旧备份
  monitor [duration] [interval]  监控系统资源 (默认60秒，5秒间隔)
  report                   生成系统报告
  update <service>         更新指定服务
  scale <service> <count>  扩缩容服务
  health                   健康检查所有服务
  help                     显示此帮助信息

示例:
  $0 start                           # 启动生产环境
  $0 logs coze-loop-python-faas 50  # 查看 Python FaaS 最近50行日志
  $0 test concurrent simple          # 执行并发测试
  $0 backup                          # 备份数据
  $0 monitor 300 10                  # 监控5分钟，每10秒采样
  $0 scale coze-loop-python-faas 3   # 扩容到3个副本

环境变量:
  DATA_DIR                 数据目录 (默认: ./data)
  BACKUP_STORAGE_PATH      备份目录 (默认: ./backups)
  ENABLE_MONITORING        启用监控服务 (默认: false)

EOF
}

# 主函数
main() {
    local command="${1:-help}"
    
    # 检查依赖
    check_dependencies
    
    case "$command" in
        "start")
            check_env_file
            load_env
            start_production
            ;;
        "stop")
            check_env_file
            load_env
            stop_production
            ;;
        "restart")
            check_env_file
            load_env
            restart_production
            ;;
        "status")
            check_env_file
            load_env
            show_service_status
            ;;
        "logs")
            check_env_file
            load_env
            show_logs "${2:-}" "${3:-100}"
            ;;
        "test")
            check_env_file
            load_env
            run_performance_test "${2:-all}" "${3:-simple}"
            ;;
        "backup")
            check_env_file
            load_env
            backup_data
            ;;
        "restore")
            if [[ -z "${2:-}" ]]; then
                log_error "请指定备份目录"
                exit 1
            fi
            check_env_file
            load_env
            restore_data "$2"
            ;;
        "cleanup-backups")
            load_env
            cleanup_old_backups
            ;;
        "monitor")
            monitor_resources "${2:-60}" "${3:-5}"
            ;;
        "report")
            check_env_file
            load_env
            generate_system_report
            ;;
        "update")
            if [[ -z "${2:-}" ]]; then
                log_error "请指定要更新的服务"
                exit 1
            fi
            check_env_file
            load_env
            update_service "$2"
            ;;
        "scale")
            if [[ -z "${2:-}" ]] || [[ -z "${3:-}" ]]; then
                log_error "请指定服务名称和副本数量"
                exit 1
            fi
            check_env_file
            load_env
            scale_service "$2" "$3"
            ;;
        "health")
            check_env_file
            load_env
            show_service_status
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            log_error "未知命令: $command"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"