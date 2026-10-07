import 'dart:convert';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../api/client.dart';
import '../api/websocket.dart';
import '../model/metric.dart';

/// Singleton WebSocket channel
final wsChannelProvider =
    Provider<MetricsWebSocketChannel>((ref) => MetricsWebSocketChannel());

/// Auto-connect WebSocket on first access
final wsConnectionProvider = FutureProvider<void>((ref) async {
  final ws = ref.watch(wsChannelProvider);
  if (!ws.isConnected) {
    await ws.connect();
  }
});

/// Provider for metrics list
final metricsProvider = FutureProvider<List<MetricInfo>>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getMetrics();
});

/// Provider for the monitored entity catalogue (securities + baskets/funds).
/// Drives the live stock list and per-stock fund membership.
final entitiesProvider = FutureProvider<List<EntityInfo>>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getEntities();
});

/// Provider for a specific metric's latest value
final metricLatestProvider =
    FutureProvider.family<MetricValue?, (String, String)>((ref, args) async {
  final (key, entity) = args;
  final client = ref.watch(apiClientProvider);
  return client.getLatest(key, entity);
});

/// StreamProvider for real-time metric updates via WebSocket
/// This enables selective rebuilds - only affected cells update when metrics change
/// On first access, fetches latest value via REST, then subscribes to WS for updates
final metricStreamProvider =
    StreamProvider.family<MetricValue, (String, String)>(
  (ref, args) async* {
    final (metricKey, entity) = args;
    final client = ref.watch(apiClientProvider);
    final ws = ref.watch(wsChannelProvider);

    // Ensure WebSocket is connected
    try {
      if (!ws.isConnected) {
        await ws.connect();
      }
    } catch (e) {
      // WS unavailable; fallback to REST polling
      final latest = await client.getLatest(metricKey, entity);
      if (latest != null) {
        yield latest;
      }
      return;
    }

    // Fetch initial value from REST
    try {
      final latest = await client.getLatest(metricKey, entity);
      if (latest != null) {
        yield latest;
      }
    } catch (e) {
      // Ignore REST errors; WS stream below will provide updates
    }

    // Subscribe to WS updates for this metric/entity pair
    // Server sends JSON: {"metric": "key", "entity": "id", "ts": "2026-06-23T...", "value": 123.45, "flag": "fresh", "tier": "daily"}
    yield* ws.stream.where((msg) {
      try {
        final json = jsonDecode(msg as String);
        return json['metric'] == metricKey && json['entity'] == entity;
      } catch (_) {
        return false;
      }
    }).map((msg) {
      final json = jsonDecode(msg as String) as Map<String, dynamic>;
      return MetricValue.fromJson(json);
    });
  },
);

/// Provider for metric series (charting)
final metricSeriesProvider =
    FutureProvider.family<List<MetricValue>, (String, String, String)>(
        (ref, args) async {
  final (key, entity, window) = args;
  final client = ref.watch(apiClientProvider);
  return client.getSeries(key, entity, window: window);
});

/// Provider for sources health
final sourcesHealthProvider = FutureProvider<List<SourceHealth>>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getSourcesHealth();
});

/// Provider for operator/system status (synthetic engine state + row counts)
final adminStatusProvider = FutureProvider<AdminStatus>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getStatus();
});

/// Provider for buy/sell/hold signal for a single entity (cached 5 min).
final signalProvider =
    FutureProvider.family<StockSignal, String>((ref, entityId) async {
  final client = ref.watch(apiClientProvider);
  return client.getSignal(entityId);
});

/// Provider for top N entities ranked by a metric's latest value.
/// Key is the metric key string (e.g. 'nimvi', 'velocity_accumulation').
final topMoversProvider =
    FutureProvider.family<List<TopMover>, String>((ref, metricKey) async {
  final client = ref.watch(apiClientProvider);
  return client.getTopMovers(metricKey, n: 15);
});

/// The [limit] securities held by the most funds, highest `fundCount` first.
///
/// Pure, deterministic derivation used by the home page ranking: stocks held
/// by no fund are excluded, and ties break by ticker id so the order is stable
/// across rebuilds (important for predictable UI and testing). Exposed as a
/// top-level function so it can be unit-tested without a live API.
List<EntityInfo> mostHeldStocks(List<EntityInfo> entities, {int limit = 50}) {
  final held = entities.where((e) => e.isSecurity && e.hasFunds).toList()
    ..sort((a, b) {
      final byCount = b.fundCount.compareTo(a.fundCount);
      return byCount != 0 ? byCount : a.id.compareTo(b.id);
    });
  if (limit > 0 && held.length > limit) return held.sublist(0, limit);
  return held;
}

/// Top [n] securities most widely held by funds (by `fundCount`, descending).
///
/// Drives the home page ranking. Derived from [entitiesProvider] — which
/// already carries each security's `fundCount` — so it adds no extra network
/// round-trip and refreshes whenever the catalogue is invalidated.
final topHeldProvider =
    FutureProvider.family<List<EntityInfo>, int>((ref, n) async {
  final entities = await ref.watch(entitiesProvider.future);
  return mostHeldStocks(entities, limit: n);
});

/// Provider for TEFAS+KAP funds that hold a given stock.
final entityFundsProvider =
    FutureProvider.family<List<FundHolding>, String>((ref, entityId) async {
  final client = ref.watch(apiClientProvider);
  return client.getEntityFunds(entityId);
});

/// How often the live fund-presence chart re-fetches. Fund holdings update at
/// most daily (when a scrape lands), so a 30s cadence keeps the chart current
/// without hammering the API; the tick is auto-disposed when no screen watches.
const presenceRefreshInterval = Duration(seconds: 30);

/// Emits an incrementing tick immediately and then every
/// [presenceRefreshInterval]. [presenceProvider] watches it to auto-refresh.
/// Auto-disposed so the timer stops once the detail screen closes.
final presenceRefreshTickProvider =
    StreamProvider.autoDispose<int>((ref) async* {
  var i = 0;
  yield i;
  yield* Stream.periodic(presenceRefreshInterval, (_) => ++i);
});

/// Real-time fund-presence series for a stock (presence % across all scraped
/// funds + daily/weekly/monthly/yearly changes). Re-fetches on every refresh
/// tick so the chart reflects newly ingested holdings without a manual reload.
/// Auto-disposed when the viewing screen is closed.
final presenceProvider = FutureProvider.autoDispose
    .family<PresenceSeries, String>((ref, entityId) async {
  ref.watch(presenceRefreshTickProvider); // rebuild on each live tick
  final client = ref.watch(apiClientProvider);
  return client.getPresence(entityId);
});

// ─── Watchlist ────────────────────────────────────────────────────────────────

const _kWatchlistKey = 'watchlist_v1';

/// Persisted watchlist of entity IDs (stock tickers).
/// Uses shared_preferences so the list survives page refreshes.
class WatchlistNotifier extends StateNotifier<Set<String>> {
  WatchlistNotifier() : super({}) {
    _load();
  }

  Future<void> _load() async {
    final prefs = await SharedPreferences.getInstance();
    final raw = prefs.getStringList(_kWatchlistKey);
    if (raw != null) state = raw.toSet();
  }

  Future<void> _save() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setStringList(_kWatchlistKey, state.toList());
  }

  void toggle(String entityId) {
    if (state.contains(entityId)) {
      state = {...state}..remove(entityId);
    } else {
      state = {...state, entityId};
    }
    _save();
  }

  bool contains(String entityId) => state.contains(entityId);
}

final watchlistProvider = StateNotifierProvider<WatchlistNotifier, Set<String>>(
  (_) => WatchlistNotifier(),
);

/// Fund scraper live progress + coverage. Auto-refreshed by AdminScreen.
final scraperStatusProvider = FutureProvider<ScraperStatus>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getScraperStatus();
});

/// Live backend activity feed (harvester / analytic / scraper), derived from
/// real DB writes. Auto-refreshed by AdminScreen so the operator can watch the
/// pipelines work.
final adminEventsProvider = FutureProvider<List<ActivityEvent>>((ref) async {
  final client = ref.watch(apiClientProvider);
  return client.getAdminEvents();
});

/// Open schema-drift alerts (persisted breaking contract changes in upstream
/// sources), breaking-first then newest. Auto-refreshed by AdminScreen; alerts
/// stay until an operator/agent resolves them.
final driftAlertsProvider = FutureProvider<List<DriftAlert>>((ref) async {
  final client = ref.watch(apiClientProvider);
  final alerts = await client.getDriftAlerts();
  return openDriftAlerts(alerts);
});
