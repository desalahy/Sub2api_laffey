import http from 'node:http'

const port = Number(process.env.PREVIEW_API_PORT || 18080)
const host = process.env.PREVIEW_API_HOST || '127.0.0.1'
const logRequests = process.env.PREVIEW_API_LOG === '1'

const now = () => new Date().toISOString()

function previewUser() {
  const timestamp = now()

  return {
    id: 1,
    username: 'Preview Admin',
    email: 'preview@example.com',
    role: 'admin',
    balance: 9999,
    concurrency: 100,
    status: 'active',
    allowed_groups: null,
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: timestamp,
    updated_at: timestamp,
    run_mode: 'standard'
  }
}

function authResponse() {
  return {
    access_token: 'preview-access-token',
    refresh_token: 'preview-refresh-token',
    expires_in: 86400,
    token_type: 'bearer',
    user: previewUser()
  }
}

function publicSettings() {
  return {
    registration_enabled: true,
    email_verify_enabled: false,
    force_email_on_third_party_signup: false,
    registration_email_suffix_whitelist: [],
    promo_code_enabled: true,
    password_reset_enabled: true,
    invitation_code_enabled: false,
    login_agreement_enabled: false,
    login_agreement_mode: 'modal',
    login_agreement_updated_at: now(),
    login_agreement_revision: 'preview',
    login_agreement_documents: [],
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: 'Laffey API',
    site_logo: '',
    site_subtitle: '订阅转 API 中转平台',
    api_base_url: '',
    contact_info: '',
    doc_url: '',
    home_content: '',
    hide_ccs_import_button: false,
    payment_enabled: true,
    risk_control_enabled: true,
    table_default_page_size: 10,
    table_page_size_options: [10, 20, 50, 100],
    custom_menu_items: [],
    custom_endpoints: [],
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    wechat_oauth_open_enabled: false,
    wechat_oauth_mp_enabled: false,
    wechat_oauth_mobile_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: '',
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    backend_mode_enabled: false,
    version: 'preview',
    balance_low_notify_enabled: false,
    account_quota_notify_enabled: false,
    balance_low_notify_threshold: 0,
    channel_monitor_enabled: true,
    channel_monitor_default_interval_seconds: 300,
    available_channels_enabled: true,
    affiliate_enabled: true
  }
}

function paginated(items = [], page = 1, pageSize = 10) {
  return {
    items,
    total: items.length,
    page,
    page_size: pageSize,
    pages: items.length ? Math.ceil(items.length / pageSize) : 0
  }
}

function zeroDashboardStats() {
  return {
    total_users: 1,
    today_new_users: 0,
    active_users: 1,
    hourly_active_users: 0,
    stats_updated_at: now(),
    stats_stale: false,
    total_api_keys: 0,
    active_api_keys: 0,
    total_accounts: 0,
    normal_accounts: 0,
    error_accounts: 0,
    ratelimit_accounts: 0,
    overload_accounts: 0,
    active_accounts: 0,
    total_channels: 0,
    active_channels: 0,
    total_requests: 0,
    total_input_tokens: 0,
    total_output_tokens: 0,
    total_cache_creation_tokens: 0,
    total_cache_read_tokens: 0,
    total_tokens: 0,
    total_cost: 0,
    total_actual_cost: 0,
    total_account_cost: 0,
    today_requests: 0,
    today_input_tokens: 0,
    today_output_tokens: 0,
    today_cache_creation_tokens: 0,
    today_cache_read_tokens: 0,
    today_tokens: 0,
    today_cost: 0,
    today_actual_cost: 0,
    today_account_cost: 0,
    uptime: 0,
    rpm: 0,
    tpm: 0,
    average_duration_ms: 0
  }
}

function zeroUserDashboardStats() {
  return {
    total_api_keys: 0,
    active_api_keys: 0,
    total_requests: 0,
    total_input_tokens: 0,
    total_output_tokens: 0,
    total_cache_creation_tokens: 0,
    total_cache_read_tokens: 0,
    total_tokens: 0,
    total_cost: 0,
    total_actual_cost: 0,
    today_requests: 0,
    today_input_tokens: 0,
    today_output_tokens: 0,
    today_cache_creation_tokens: 0,
    today_cache_read_tokens: 0,
    today_tokens: 0,
    today_cost: 0,
    today_actual_cost: 0,
    average_duration_ms: 0,
    rpm: 0,
    tpm: 0
  }
}

function paymentMethodLimit() {
  return {
    currency: 'USD',
    daily_limit: 0,
    daily_used: 0,
    daily_remaining: 0,
    single_min: 1,
    single_max: 1000,
    fee_rate: 0,
    available: true
  }
}

function paymentConfig() {
  return {
    payment_enabled: true,
    enabled: true,
    min_amount: 1,
    max_amount: 1000,
    daily_limit: 0,
    max_pending_orders: 5,
    order_timeout_minutes: 15,
    balance_disabled: false,
    balance_recharge_multiplier: 1,
    enabled_payment_types: ['alipay'],
    load_balance_strategy: 'round-robin',
    product_name_prefix: '',
    product_name_suffix: '',
    help_image_url: '',
    help_text: '',
    stripe_publishable_key: ''
  }
}

function checkoutInfo() {
  return {
    methods: {
      alipay: paymentMethodLimit()
    },
    global_min: 1,
    global_max: 1000,
    plans: [],
    balance_disabled: false,
    balance_recharge_multiplier: 1,
    recharge_fee_rate: 0,
    help_text: '',
    help_image_url: '',
    stripe_publishable_key: ''
  }
}

function paymentDashboard() {
  return {
    today_amount: 0,
    total_amount: 0,
    today_count: 0,
    total_count: 0,
    avg_amount: 0,
    daily_series: [],
    payment_methods: [],
    top_users: []
  }
}

function opsOverview() {
  const timestamp = now()

  return {
    start_time: timestamp,
    end_time: timestamp,
    platform: 'all',
    group_id: null,
    health_score: 100,
    system_metrics: null,
    job_heartbeats: [],
    success_count: 0,
    error_count_total: 0,
    business_limited_count: 0,
    error_count_sla: 0,
    request_count_total: 0,
    request_count_sla: 0,
    token_consumed: 0,
    sla: 1,
    error_rate: 0,
    upstream_error_rate: 0,
    upstream_error_count_excl_429_529: 0,
    upstream_429_count: 0,
    upstream_529_count: 0,
    qps: { current: 0, peak: 0, avg: 0 },
    tps: { current: 0, peak: 0, avg: 0 },
    duration: { p50_ms: 0, p90_ms: 0, p95_ms: 0, p99_ms: 0, avg_ms: 0, max_ms: 0 },
    ttft: { p50_ms: 0, p90_ms: 0, p95_ms: 0, p99_ms: 0, avg_ms: 0, max_ms: 0 }
  }
}

function opsThroughputTrend() {
  return {
    bucket: 'minute',
    points: [],
    by_platform: [],
    top_groups: []
  }
}

function opsErrorTrend() {
  return {
    bucket: 'minute',
    points: []
  }
}

function opsAdvancedSettings() {
  return {
    data_retention: {
      cleanup_enabled: false,
      cleanup_schedule: '',
      error_log_retention_days: 30,
      minute_metrics_retention_days: 7,
      hourly_metrics_retention_days: 30
    },
    aggregation: {
      aggregation_enabled: false
    },
    ignore_count_tokens_errors: true,
    ignore_context_canceled: true,
    ignore_no_available_accounts: true,
    ignore_invalid_api_key_errors: true,
    ignore_insufficient_balance_errors: true,
    display_openai_token_stats: false,
    display_alert_events: false,
    auto_refresh_enabled: false,
    auto_refresh_interval_seconds: 30
  }
}

function opsAlertSettings() {
  return {
    evaluation_interval_seconds: 60,
    distributed_lock: { enabled: false, ttl_seconds: 60 },
    silencing: {
      enabled: false,
      global_until_rfc3339: '',
      global_reason: '',
      entries: []
    },
    thresholds: {}
  }
}

function defaultRange(searchParams) {
  return {
    start_date: searchParams.get('start_date') || new Date().toISOString().slice(0, 10),
    end_date: searchParams.get('end_date') || new Date().toISOString().slice(0, 10),
    granularity: searchParams.get('granularity') || 'day'
  }
}

function configObject() {
  return {
    smtp: {},
    oauth: {},
    payment: {},
    proxy: {},
    system: {},
    risk_control: {},
    channel_monitor: {}
  }
}

function unwrapPath(pathname) {
  return pathname.replace(/^\/api\/v1/, '')
}

function responseFor(method, requestUrl) {
  const url = new URL(requestUrl, `http://${host}:${port}`)
  const path = unwrapPath(url.pathname)
  const page = Number(url.searchParams.get('page') || 1)
  const pageSize = Number(url.searchParams.get('page_size') || 10)
  const range = defaultRange(url.searchParams)

  if (method === 'OPTIONS') return null

  if (path === '/settings/public') return publicSettings()

  if (path === '/auth/login' || path === '/auth/register' || path === '/auth/refresh') {
    return authResponse()
  }

  if (path === '/auth/me') return previewUser()
  if (path === '/auth/logout') return { message: 'ok' }

  if (path === '/payment/config' || path === '/admin/payment/config') return paymentConfig()
  if (path === '/payment/checkout-info') return checkoutInfo()
  if (path === '/payment/limits') {
    return { methods: checkoutInfo().methods, global_min: 1, global_max: 1000 }
  }
  if (path === '/payment/plans' || path === '/payment/channels' || path === '/admin/payment/plans' || path === '/admin/payment/channels' || path === '/admin/payment/providers') return []
  if (path === '/payment/orders/my' || path === '/admin/payment/orders') return paginated([], page, pageSize)
  if (path === '/payment/orders/refund-eligible-providers') return { provider_instance_ids: [] }
  if (path === '/admin/payment/dashboard') return paymentDashboard()

  if (path === '/admin/ops/settings/advanced') return opsAdvancedSettings()
  if (path === '/admin/ops/alerts/settings') return opsAlertSettings()
  if (path === '/admin/ops/dashboard/overview') return opsOverview()
  if (path === '/admin/ops/dashboard/snapshot-v2') {
    return {
      generated_at: now(),
      overview: opsOverview(),
      throughput_trend: opsThroughputTrend(),
      error_trend: opsErrorTrend()
    }
  }
  if (path === '/admin/ops/dashboard/throughput-trend') return opsThroughputTrend()
  if (path === '/admin/ops/dashboard/latency-histogram') {
    return {
      start_time: now(),
      end_time: now(),
      platform: 'all',
      group_id: null,
      total_requests: 0,
      buckets: []
    }
  }
  if (path === '/admin/ops/dashboard/error-trend') return opsErrorTrend()
  if (path === '/admin/ops/dashboard/error-distribution') return { total: 0, items: [] }
  if (path === '/admin/ops/dashboard/openai-token-stats') {
    return {
      time_range: '1h',
      start_time: now(),
      end_time: now(),
      platform: 'all',
      group_id: null,
      items: [],
      total: 0,
      page: page,
      page_size: pageSize
    }
  }
  if (path.startsWith('/admin/ops/')) return paginated([], page, pageSize)

  if (path === '/admin/dashboard/stats') return zeroDashboardStats()
  if (path === '/admin/dashboard/realtime') {
    return {
      active_requests: 0,
      requests_per_minute: 0,
      average_response_time: 0,
      error_rate: 0
    }
  }
  if (path === '/admin/dashboard/snapshot-v2') {
    return {
      generated_at: now(),
      ...range,
      stats: { ...zeroDashboardStats(), uptime: 0 },
      trend: [],
      models: [],
      groups: [],
      users_trend: []
    }
  }
  if (path === '/admin/dashboard/trend' || path === '/admin/dashboard/api-keys-trend' || path === '/admin/dashboard/users-trend') {
    return { trend: [], ...range }
  }
  if (path === '/admin/dashboard/models') return { models: [], ...range }
  if (path === '/admin/dashboard/groups') return { groups: [], ...range }
  if (path === '/admin/dashboard/user-breakdown') return { users: [], ...range }
  if (path === '/admin/dashboard/users-ranking') {
    return {
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: range.start_date,
      end_date: range.end_date
    }
  }
  if (path === '/admin/dashboard/users-usage' || path === '/admin/dashboard/api-keys-usage') {
    return { stats: {} }
  }

  if (path === '/usage/dashboard/stats') return zeroUserDashboardStats()
  if (path === '/usage/dashboard/trend') return { trend: [], ...range }
  if (path === '/usage/dashboard/models') return { models: [], ...range }
  if (path === '/usage/dashboard/api-keys-usage') return { stats: {} }
  if (path === '/usage/stats') {
    return {
      total_requests: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
      cache_creation_tokens: 0,
      cache_read_tokens: 0
    }
  }

  if (path === '/subscriptions/summary') {
    return {
      active_count: 0,
      subscriptions: []
    }
  }
  if (path === '/subscriptions' || path === '/subscriptions/active' || path === '/subscriptions/progress') {
    return []
  }

  if (path === '/groups/available') return []
  if (path === '/groups/rates') return {}

  if (path === '/user/affiliate') {
    return {
      user_id: 1,
      aff_code: 'PREVIEW',
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 0,
      invitees: []
    }
  }

  if (path.endsWith('/admin-api-key/status')) {
    return {
      exists: true,
      masked_key: 'sk-preview-****'
    }
  }
  if (path.endsWith('/admin-api-key')) return { api_key: 'sk-preview-local' }
  if (path.includes('/settings') && method === 'GET') return configObject()

  if (path.includes('/health') || path.includes('/status')) return { status: 'ok' }
  if (path.includes('/models')) return []
  if (path.includes('/trend')) return { trend: [], ...range }
  if (path.includes('/stats')) return { ...zeroUserDashboardStats(), ...zeroDashboardStats() }
  if (method === 'GET') return paginated([], page, pageSize)

  return { message: 'preview ok' }
}

const server = http.createServer((req, res) => {
  if (logRequests) {
    console.log(`${new Date().toISOString()} ${req.method || 'GET'} ${req.url || '/'}`)
  }

  const data = responseFor(req.method || 'GET', req.url || '/')

  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Access-Control-Allow-Headers', 'Content-Type, Authorization')
  res.setHeader('Access-Control-Allow-Methods', 'GET,POST,PUT,PATCH,DELETE,OPTIONS')

  if (req.method === 'OPTIONS') {
    res.writeHead(204)
    res.end()
    return
  }

  res.setHeader('Content-Type', 'application/json; charset=utf-8')
  res.writeHead(200)
  res.end(JSON.stringify({ code: 0, message: 'ok', data }))
})

server.listen(port, host, () => {
  if (logRequests) {
    console.log(`Frontend preview mock API listening at http://${host}:${port}`)
  }
})
