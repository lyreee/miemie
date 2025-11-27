#!/bin/bash

# 消息系统压力测试脚本
# 测试30用户，每人100条数据

set -e

# 配置参数
API_BASE="http://localhost:8080"
TOTAL_USERS=30
MESSAGES_PER_USER=100
CONCURRENT_REQUESTS=10  # 并发请求数

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 进度条
show_progress() {
    local current=$1
    local total=$2
    local desc=$3
    local percent=$((current * 100 / total))
    local filled=$((percent / 2))
    local empty=$((50 - filled))

    printf "\r%s: [" "$desc"
    printf "%*s" $filled | tr ' ' '='
    printf "%*s" $empty | tr ' ' '-'
    printf "] %d%% (%d/%d)" $percent $current $total
}

# 生成随机消息数据
generate_message_data() {
    local user_id=$1
    local msg_num=$2

    # 随机选择消息类型
    local message_types=("text" "notification" "alert" "system")
    local msg_type=${message_types[$((RANDOM % ${#message_types[@]}))]}

    # 随机选择频道
    local channels=("default" "notifications" "alerts" "updates" "events")
    local channel=${channels[$((RANDOM % ${#channels[@]}))]}

    # 随机优先级 (1-10)
    local priority=$((RANDOM % 10 + 1))

    # 随机发送者
    local senders=("auto_tester" "stress_bot" "msg_generator" "test_client" "api_client")
    local sender=${senders[$((RANDOM % ${#senders[@]}))]}

    cat <<EOF
{
    "channel_id": "$channel",
    "title": "压力测试消息$msg_num",
    "content": "这是用户$user_id的第$msg_num条压力测试消息，时间戳:$(date +%s)，随机数:$RANDOM",
    "message_type": "$msg_type",
    "priority": $priority,
    "sender": "$sender"
}
EOF
}

# 发送单条消息
send_message() {
    local user_id=$1
    local msg_num=$2
    local message_data

    message_data=$(generate_message_data "$user_id" "$msg_num")

    local response=$(curl -s -w "%{http_code}" \
        -X POST "$API_BASE/api/v3/messages" \
        -H "Content-Type: application/json" \
        -H "User-ID: $user_id" \
        -d "$message_data")

    local http_code="${response: -3}"

    if [[ "$http_code" == "202" ]]; then
        return 0  # 成功
    else
        return 1  # 失败
    fi
}

# 批量发送消息
send_batch_messages() {
    local user_id=$1
    local start_msg=$2
    local end_msg=$3
    local local_success=0
    local local_failed=0

    log_info "用户 $user_id: 发送消息 $start_msg - $end_msg"

    for ((i=start_msg; i<=end_msg; i++)); do
        if send_message "$user_id" "$i"; then
            ((local_success++))
        else
            ((local_failed++))
        fi

        # 显示进度
        show_progress $((i - start_msg + 1)) $((end_msg - start_msg + 1)) "用户 $user_id"

        # 每20条消息暂停一下，减少等待时间
        if ((i % 20 == 0)); then
            sleep 0.05
        fi
    done
    echo  # 换行

    echo "用户 $user_id 完成: 成功 $local_success, 失败 $local_failed"
    return $local_failed
}

# 并发发送函数
concurrent_send() {
    local pids=()

    # 启动并发进程
    for ((i=1; i<=CONCURRENT_REQUESTS; i++)); do
        {
            local user_id="stress_user_$i"
            local start_msg=$(((i-1) * (MESSAGES_PER_USER / CONCURRENT_REQUESTS) + 1))
            local end_msg=$((i * (MESSAGES_PER_USER / CONCURRENT_REQUESTS)))

            send_batch_messages "$user_id" "$start_msg" "$end_msg"
        } &
        pids+=($!)
    done

    # 等待所有并发进程完成
    local failed_count=0
    for pid in "${pids[@]}"; do
        wait "$pid"
        local exit_code=$?
        if [[ $exit_code -ne 0 ]]; then
            ((failed_count++))
        fi
    done

    return $failed_count
}

# 获取系统统计
get_system_stats() {
    echo "=== 系统统计信息 ==="

    # 获取投递系统统计
    echo "投递系统统计:"
    curl -s "$API_BASE/api/v3/delivery/stats" | jq '.data' || echo "获取投递统计失败"

    echo
    echo "缓存系统统计:"
    curl -s "$API_BASE/api/v3/workspace/cache/stats" | jq '.data' || echo "获取缓存统计失败"

    echo
    echo "数据库文件统计:"
    local db_count=$(find /home/lyre/miemie/data/messages.db -name "*.db" | wc -l)
    local wal_count=$(find /home/lyre/miemie/data/messages.db -name "*-wal" | wc -l)
    local shm_count=$(find /home/lyre/miemie/data/messages.db -name "*-shm" | wc -l)
    echo "数据库文件: $db_count"
    echo "WAL文件: $wal_count"
    echo "SHM文件: $shm_count"

    echo
    echo "用户目录统计:"
    local user_count=$(find /home/lyre/miemie/data/messages.db -maxdepth 1 -type d | grep -v "^/home/lyre/miemie/data/messages.db$" | wc -l)
    echo "用户目录数: $user_count"
}

# 主测试函数
main() {
    echo "============================================"
    echo "🚀 消息系统压力测试开始"
    echo "============================================"
    echo "测试配置:"
    echo "  - 用户数量: $TOTAL_USERS"
    echo "  - 每用户消息数: $MESSAGES_PER_USER"
    echo "  - 总消息数: $((TOTAL_USERS * MESSAGES_PER_USER))"
    echo "  - 并发请求数: $CONCURRENT_REQUESTS"
    echo "  - API地址: $API_BASE"
    echo "============================================"
    echo

    # 检查系统状态
    log_info "检查系统状态..."
    if ! curl -s "$API_BASE/health" > /dev/null; then
        log_error "系统未运行，请先启动服务"
        exit 1
    fi
    log_success "系统状态正常"

    # 记录开始时间
    local start_time=$(date +%s)

    echo
    log_info "开始压力测试..."

    # 执行并发压力测试
    local total_failed=0
    local batch_size=$((TOTAL_USERS / CONCURRENT_REQUESTS))
    local remaining_users=$((TOTAL_USERS % CONCURRENT_REQUESTS))

    # 主批量处理 - 真正的并发执行
    for ((batch=1; batch<=batch_size; batch++)); do
        echo "执行第 $batch 批次测试..."

        local start_user=$(((batch-1) * CONCURRENT_REQUESTS + 1))
        local end_user=$((batch * CONCURRENT_REQUESTS))

        # 使用后台进程并发执行
        local pids=()
        local temp_failed=0

        for ((i=1; i<=CONCURRENT_REQUESTS; i++)); do
            local current_user=$((start_user + i - 1))

            # 每个用户的消息编号从1到MESSAGES_PER_USER
            {
                send_batch_messages "stress_user_$current_user" "1" "$MESSAGES_PER_USER"
                local batch_failed=$?
                if [[ $batch_failed -ne 0 ]]; then
                    echo "用户 stress_user_$current_user 有 $batch_failed 条消息失败"
                fi
            } &
            pids+=($!)
        done

        # 等待所有并发进程完成
        for pid in "${pids[@]}"; do
            wait "$pid"
            local exit_code=$?
            if [[ $exit_code -ne 0 ]]; then
                ((temp_failed++))
            fi
        done

        total_failed=$((total_failed + temp_failed))

        echo "第 $batch 批次完成，失败用户数: $temp_failed"
        echo

        # 每批次后稍作停顿
        sleep 1
    done

    # 处理剩余用户
    if [[ $remaining_users -gt 0 ]]; then
        echo "处理剩余 $remaining_users 个用户..."
        for ((i=1; i<=remaining_users; i++)); do
            local current_user=$((batch_size * CONCURRENT_REQUESTS + i))

            send_batch_messages "stress_user_$current_user" "1" "$MESSAGES_PER_USER"
            local batch_failed=$?

            if [[ $batch_failed -ne 0 ]]; then
                ((total_failed++))
            fi
        done
    fi

    # 记录结束时间
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo
    echo "============================================"
    echo "📊 压力测试结果"
    echo "============================================"

    local total_messages=$((TOTAL_USERS * MESSAGES_PER_USER))
    local successful_messages=$((total_messages - total_failed))
    local success_rate=$((successful_messages * 100 / total_messages))
    local messages_per_second=$((successful_messages / duration))

    echo "测试时长: ${duration}秒"
    echo "总消息数: $total_messages"
    echo "成功消息: $successful_messages"
    echo "失败消息: $total_failed"
    echo "成功率: ${success_rate}%"
    echo "平均处理速度: ${messages_per_second} 消息/秒"

    if [[ $total_failed -eq 0 ]]; then
        log_success "压力测试完全成功！"
    else
        log_warning "压力测试完成，但有 $total_failed 条消息失败"
    fi

    echo
    get_system_stats

    echo
    echo "============================================"
    echo "✅ 压力测试完成"
    echo "============================================"
}

# 脚本入口
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    # 检查依赖
    if ! command -v curl &> /dev/null; then
        log_error "需要安装 curl"
        exit 1
    fi

    if ! command -v jq &> /dev/null; then
        log_warning "建议安装 jq 以获得更好的统计显示"
    fi

    # 检查服务是否运行
    if ! curl -s "http://localhost:8080/health" &> /dev/null; then
        log_error "服务未运行，请先启动: ./miemie"
        exit 1
    fi

    # 执行测试
    main "$@"
fi