import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../model/metric.dart';

/// Builds the absolute API base URL from the current page origin.
/// Using an absolute URL (not a relative path) avoids browser-specific
/// differences in how XHR resolves relative URLs — the root cause of
/// Firefox failing to reach /api/v1/ endpoints.
String _apiBaseUrl() {
  final b = Uri.base;
  final port =
      (b.port == 0 || b.port == 80 || b.port == 443) ? '' : ':${b.port}';
  return '${b.scheme}://${b.host}$port/api/v1';
}

/// REST API client for neyialiyorlar metrics
class ApiClient {
  ApiClient({String? baseUrl})
      : _dio = Dio(BaseOptions(
          baseUrl: baseUrl ?? _apiBaseUrl(),
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 30),
        ));
  final Dio _dio;

  /// Fetch list of all available metrics
  Future<List<MetricInfo>> getMetrics() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/metrics');
      final body = response.data ?? {};
      final metrics = (body['metrics'] as List<dynamic>?)
              ?.map((m) => MetricInfo.fromJson(m as Map<String, dynamic>))
              .toList() ??
          [];
      return metrics;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch the catalogue of monitored entities (securities + baskets/funds).
  /// Powers the live stock list and each security's fund membership.
  Future<List<EntityInfo>> getEntities() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/entities');
      final body = response.data ?? {};
      final entities = (body['entities'] as List<dynamic>?)
              ?.map((e) => EntityInfo.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [];
      return entities;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch latest metric value for an entity
  Future<MetricValue?> getLatest(String key, String entity) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/metrics/$key/latest',
        queryParameters: {'entity': entity},
      );
      final body = response.data ?? {};
      if (response.statusCode == 404) {
        return null;
      }
      return MetricValue.fromJson(body);
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) {
        return null;
      }
      throw _handleError(e);
    }
  }

  /// Fetch metric series for charting
  Future<List<MetricValue>> getSeries(String key, String entity,
      {String window = '30d'}) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/metrics/$key/series',
        queryParameters: {'entity': entity, 'win': window},
      );
      final body = response.data ?? {};
      final points = (body['points'] as List<dynamic>?)
              ?.map((p) => MetricValue.fromJson(p as Map<String, dynamic>))
              .toList() ??
          [];
      return points;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch source health status
  Future<List<SourceHealth>> getSourcesHealth() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/sources/health');
      final body = response.data ?? {};
      final sources = (body['sources'] as List<dynamic>?)
              ?.map((s) => SourceHealth.fromJson(s as Map<String, dynamic>))
              .toList() ??
          [];
      return sources;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch operator/system status: synthetic engine state + live row counts.
  /// Powers the "investigate my system" view.
  Future<AdminStatus> getStatus() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/admin/status');
      return AdminStatus.fromJson(response.data ?? {});
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch fund scraper run progress + coverage stats.
  Future<ScraperStatus> getScraperStatus() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/admin/scraper');
      return ScraperStatus.fromJson(response.data ?? {});
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch the live backend activity feed (harvester / analytic / scraper),
  /// derived from real DB writes. Powers the "Canlı Olaylar" panel.
  Future<List<ActivityEvent>> getAdminEvents() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/admin/events');
      final body = response.data ?? {};
      return (body['events'] as List<dynamic>?)
              ?.map((e) => ActivityEvent.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [];
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch the open schema-drift alerts (persisted breaking contract changes in
  /// upstream sources). Powers the "Şema Kayması Uyarıları" panel; alerts stay
  /// until resolved.
  Future<List<DriftAlert>> getDriftAlerts() async {
    try {
      final response = await _dio.get<Map<String, dynamic>>('/admin/drift');
      final body = response.data ?? {};
      return (body['alerts'] as List<dynamic>?)
              ?.map((a) => DriftAlert.fromJson(a as Map<String, dynamic>))
              .toList() ??
          [];
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Mark a drift alert resolved once the upstream schema change is handled.
  /// Returns true on success, false when the alert no longer exists or is
  /// already resolved (HTTP 404).
  Future<bool> resolveDriftAlert(int id) async {
    try {
      final response =
          await _dio.post<Map<String, dynamic>>('/admin/drift/$id/resolve');
      return (response.data?['resolved'] as bool?) ?? true;
    } on DioException catch (e) {
      if (e.response?.statusCode == 404) {
        return false;
      }
      throw _handleError(e);
    }
  }

  /// Fetch the buy/sell/hold signal for a BIST-listed entity.
  Future<StockSignal> getSignal(String entityId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/entities/$entityId/signal',
      );
      return StockSignal.fromJson(response.data ?? {});
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch top N entities by latest value for a metric.
  /// [order] is 'desc' (default) or 'asc'.
  Future<List<TopMover>> getTopMovers(String key,
      {int n = 15, String order = 'desc'}) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/metrics/$key/top',
        queryParameters: {'n': n, 'order': order},
      );
      final body = response.data ?? {};
      return (body['movers'] as List<dynamic>?)
              ?.map((m) => TopMover.fromJson(m as Map<String, dynamic>))
              .toList() ??
          [];
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch funds that hold a given BIST stock (TEFAS + KAP cross-validated).
  Future<List<FundHolding>> getEntityFunds(String entityId) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/entities/$entityId/funds',
      );
      final body = response.data ?? {};
      return (body['funds'] as List<dynamic>?)
              ?.map((f) => FundHolding.fromJson(f as Map<String, dynamic>))
              .toList() ??
          [];
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Fetch a stock's fund-presence time series + d/w/m/y changes.
  /// [win] selects the chart window (1m, 3m, 6m, 1y, 2y, all); the d/w/m/y
  /// changes are always computed against the appropriate baseline snapshot.
  Future<PresenceSeries> getPresence(String entityId,
      {String win = '1y'}) async {
    try {
      final response = await _dio.get<Map<String, dynamic>>(
        '/entities/$entityId/presence',
        queryParameters: {'win': win},
      );
      return PresenceSeries.fromJson(response.data ?? {});
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Trigger one synthetic recompute round (dev "tweak/test" control).
  /// Returns the number of values emitted. Throws if the engine is disabled
  /// (server responds 501).
  Future<int> tick() async {
    try {
      final response = await _dio.post<Map<String, dynamic>>('/admin/tick');
      final body = response.data ?? {};
      return (body['emitted'] as num?)?.toInt() ?? 0;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Batch query multiple metrics
  Future<List<MetricValue>> queryMetrics(List<QueryRequest> queries) async {
    try {
      final requestBody = {
        'queries': queries
            .map((q) => {
                  'metric': q.metric,
                  'entities': q.entities,
                })
            .toList(),
      };
      final response = await _dio.post<Map<String, dynamic>>(
        '/query',
        data: requestBody,
      );
      final body = response.data ?? {};
      final result = (body['result'] as List<dynamic>?)
              ?.map((r) => MetricValue.fromJson(r as Map<String, dynamic>))
              .toList() ??
          [];
      return result;
    } on DioException catch (e) {
      throw _handleError(e);
    }
  }

  /// Handle DioException and re-throw as readable error
  Exception _handleError(DioException e) {
    final message = e.response?.data['error'] as String? ??
        e.message ??
        'Unknown API error';
    return Exception('API Error: $message');
  }
}

class QueryRequest {
  QueryRequest({required this.metric, required this.entities});
  final String metric;
  final List<String> entities;
}

/// Operator/system status returned by GET /api/v1/admin/status.
class AdminStatus {
  AdminStatus({
    required this.service,
    required this.time,
    required this.syntheticEnabled,
    required this.metricsCatalog,
    required this.counts,
  });

  factory AdminStatus.fromJson(Map<String, dynamic> json) {
    final rawCounts = (json['counts'] as Map<String, dynamic>?) ?? {};
    return AdminStatus(
      service: json['service'] as String? ?? 'api',
      time: DateTime.parse(json['time'] as String),
      syntheticEnabled: json['synthetic_enabled'] as bool? ?? false,
      metricsCatalog: (json['metrics_catalog'] as num?)?.toInt() ?? 0,
      counts:
          rawCounts.map((key, value) => MapEntry(key, (value as num).toInt())),
    );
  }

  final String service;
  final DateTime time;
  final bool syntheticEnabled;
  final int metricsCatalog;
  final Map<String, int> counts;
}

class SourceHealth {
  SourceHealth({
    required this.source,
    required this.checkedAt,
    required this.breakerOpen,
    required this.consecutiveFailures,
    this.lastOkAt,
  });

  factory SourceHealth.fromJson(Map<String, dynamic> json) => SourceHealth(
        source: json['source'] as String,
        checkedAt: DateTime.parse(json['checked_at'] as String),
        breakerOpen: json['breaker_open'] as bool,
        consecutiveFailures: json['consecutive_failures'] as int,
        lastOkAt: json['last_ok_at'] != null
            ? DateTime.parse(json['last_ok_at'] as String)
            : null,
      );

  final String source;
  final DateTime? lastOkAt;
  final DateTime checkedAt;
  final bool breakerOpen;
  final int consecutiveFailures;

  Map<String, dynamic> toJson() => {
        'source': source,
        'checked_at': checkedAt.toIso8601String(),
        'breaker_open': breakerOpen,
        'consecutive_failures': consecutiveFailures,
        'last_ok_at': lastOkAt?.toIso8601String(),
      };
}

/// Provider for the API client (singleton)
final apiClientProvider = Provider((ref) => ApiClient());
