enum Confidence {
  fresh,
  stale,
  approx,
}

enum CadenceTier {
  intraday,
  daily,
  weekly,
  event,
}

class MetricValue {
  MetricValue({
    required this.metric,
    required this.entity,
    required this.ts,
    required this.flag,
    required this.tier,
    this.value,
  });

  factory MetricValue.fromJson(Map<String, dynamic> json) => MetricValue(
        metric: json['metric'] as String,
        entity: json['entity'] as String,
        ts: DateTime.parse(json['ts'] as String),
        value: (json['value'] as num?)?.toDouble(),
        flag: Confidence.values.firstWhere(
          (e) => e.name == json['flag'],
          orElse: () => Confidence.fresh,
        ),
        tier: CadenceTier.values.firstWhere(
          (e) => e.name == json['tier'],
          orElse: () => CadenceTier.daily,
        ),
      );

  final String metric;
  final String entity;
  final DateTime ts;
  final double? value;
  final Confidence flag;
  final CadenceTier tier;

  Map<String, dynamic> toJson() => {
        'metric': metric,
        'entity': entity,
        'ts': ts.toIso8601String(),
        'value': value,
        'flag': flag.name,
        'tier': tier.name,
      };
}

class MetricInfo {
  MetricInfo({
    required this.key,
    required this.name,
    required this.flag,
    required this.tier,
  });

  factory MetricInfo.fromJson(Map<String, dynamic> json) => MetricInfo(
        key: json['key'] as String,
        name: json['name'] as String,
        flag: Confidence.values.firstWhere(
          (e) => e.name == json['flag'],
          orElse: () => Confidence.fresh,
        ),
        tier: CadenceTier.values.firstWhere(
          (e) => e.name == json['tier'],
          orElse: () => CadenceTier.daily,
        ),
      );

  final String key;
  final String name;
  final Confidence flag;
  final CadenceTier tier;

  /// Human-readable label for UI. Falls back to a prettified [key] when the
  /// catalogue `name` is empty (the synthetic seed leaves it blank).
  String get displayName {
    if (name.isNotEmpty) {
      return name;
    }
    return key
        .split('_')
        .where((w) => w.isNotEmpty)
        .map((w) => '${w[0].toUpperCase()}${w.substring(1)}')
        .join(' ');
  }

  Map<String, dynamic> toJson() => {
        'key': key,
        'name': name,
        'flag': flag.name,
        'tier': tier.name,
      };
}

/// A monitored entity: an individual security (stock) or a basket/fund.
/// Mirrors the `/api/v1/entities` catalogue. `baskets` lists all BIST index
/// baskets (XU030, XU100, etc.) this security belongs to.
class EntityInfo {
  EntityInfo({
    required this.id,
    required this.type,
    required this.displayTicker,
    this.isin,
    this.basket,
    this.baskets = const [],
    this.fundCount = 0,
  });

  factory EntityInfo.fromJson(Map<String, dynamic> json) => EntityInfo(
        id: json['id'] as String,
        type: json['type'] as String? ?? 'security',
        displayTicker:
            json['display_ticker'] as String? ?? json['id'] as String,
        isin: json['isin'] as String?,
        basket: json['basket'] as String?,
        baskets: (json['baskets'] as List<dynamic>?)
                ?.map((e) => e as String)
                .toList() ??
            const [],
        fundCount: (json['fund_count'] as num?)?.toInt() ?? 0,
      );

  final String id;
  final String type;
  final String displayTicker;
  final String? isin;
  final String? basket; // first basket (back-compat)
  final List<String> baskets; // all index/fund memberships
  final int fundCount; // number of funds holding this security (0 = none)

  /// True when this entity is an individual tradeable security (a stock).
  bool get isSecurity => type == 'security';

  /// True when at least one fund is known to hold this security.
  bool get hasFunds => fundCount > 0;

  Map<String, dynamic> toJson() => {
        'id': id,
        'type': type,
        'display_ticker': displayTicker,
        if (isin != null) 'isin': isin,
        if (basket != null) 'basket': basket,
        if (baskets.isNotEmpty) 'baskets': baskets,
        'fund_count': fundCount,
      };
}

class ListMetricsResponse {
  ListMetricsResponse({required this.metrics});

  factory ListMetricsResponse.fromJson(Map<String, dynamic> json) =>
      ListMetricsResponse(
        metrics: (json['metrics'] as List<dynamic>)
            .map((m) => MetricInfo.fromJson(m as Map<String, dynamic>))
            .toList(),
      );

  final List<MetricInfo> metrics;

  Map<String, dynamic> toJson() => {
        'metrics': metrics.map((m) => m.toJson()).toList(),
      };
}

class GetSeriesResponse {
  GetSeriesResponse({
    required this.metric,
    required this.entity,
    required this.window,
    required this.points,
  });

  factory GetSeriesResponse.fromJson(Map<String, dynamic> json) =>
      GetSeriesResponse(
        metric: json['metric'] as String,
        entity: json['entity'] as String,
        window: json['window'] as String,
        points: (json['points'] as List<dynamic>)
            .map((p) => MetricValue.fromJson(p as Map<String, dynamic>))
            .toList(),
      );

  final String metric;
  final String entity;
  final String window;
  final List<MetricValue> points;

  Map<String, dynamic> toJson() => {
        'metric': metric,
        'entity': entity,
        'window': window,
        'points': points.map((p) => p.toJson()).toList(),
      };
}

/// Directional trading signal for a BIST-listed security.
/// Computed from 5-day vs 20-day simple moving average on real closing prices.
enum SignalDirection { BUY, SELL, HOLD, INSUFFICIENT_DATA }

class StockSignal {
  StockSignal({
    required this.entityId,
    required this.signal,
    required this.confidence,
    required this.score,
    required this.ma5,
    required this.ma20,
    required this.price,
    required this.reason,
    required this.computedAt,
  });

  factory StockSignal.fromJson(Map<String, dynamic> json) => StockSignal(
        entityId: json['entity_id'] as String,
        signal: _parseSignal(json['signal'] as String? ?? ''),
        confidence: Confidence.values.firstWhere(
          (e) => e.name == (json['confidence'] as String? ?? 'stale'),
          orElse: () => Confidence.stale,
        ),
        score: (json['score'] as num?)?.toDouble() ?? 0,
        ma5: (json['ma5'] as num?)?.toDouble() ?? 0,
        ma20: (json['ma20'] as num?)?.toDouble() ?? 0,
        price: (json['price'] as num?)?.toDouble() ?? 0,
        reason: json['reason'] as String? ?? '',
        computedAt: DateTime.tryParse(json['computed_at'] as String? ?? '') ??
            DateTime.now(),
      );

  final String entityId;
  final SignalDirection signal;
  final Confidence confidence;
  final double score;
  final double ma5;
  final double ma20;
  final double price;
  final String reason;
  final DateTime computedAt;

  static SignalDirection _parseSignal(String s) {
    switch (s) {
      case 'BUY':
        return SignalDirection.BUY;
      case 'SELL':
        return SignalDirection.SELL;
      case 'HOLD':
        return SignalDirection.HOLD;
      default:
        return SignalDirection.INSUFFICIENT_DATA;
    }
  }
}

/// Single entry in the top-N ranking returned by GET /api/v1/metrics/{key}/top.
class TopMover {
  TopMover({required this.entity, required this.flag, this.value});

  factory TopMover.fromJson(Map<String, dynamic> json) => TopMover(
        entity: json['entity'] as String,
        value: (json['value'] as num?)?.toDouble(),
        flag: Confidence.values.firstWhere(
          (e) => e.name == (json['flag'] as String? ?? ''),
          orElse: () => Confidence.approx,
        ),
      );

  final String entity;
  final double? value;
  final Confidence flag;
}

/// A fund (TEFAS EMK) that holds a given BIST stock.
class FundHolding {
  FundHolding({
    required this.fundCode,
    required this.fundTitle,
    required this.fundManager,
    required this.fundType,
    required this.asOfDate,
    required this.source,
    this.weightPct,
  });

  factory FundHolding.fromJson(Map<String, dynamic> json) => FundHolding(
        fundCode: json['fund_code'] as String,
        fundTitle: json['fund_title'] as String,
        fundManager: json['fund_manager'] as String,
        fundType: json['fund_type'] as String,
        weightPct: (json['weight_pct'] as num?)?.toDouble(),
        asOfDate: json['as_of_date'] as String,
        source: json['source'] as String,
      );

  final String fundCode;
  final String fundTitle;
  final String fundManager;
  final String fundType;
  final double? weightPct;
  final String asOfDate;
  final String source; // 'tefas' | 'kap'
}

/// One snapshot of a stock's presence across funds: on [date], [funds] of
/// [total] reporting funds held the stock, so [pct] = funds / total * 100.
class PresencePoint {
  PresencePoint({
    required this.date,
    required this.pct,
    required this.funds,
    required this.total,
  });

  factory PresencePoint.fromJson(Map<String, dynamic> json) => PresencePoint(
        date: DateTime.parse(json['date'] as String),
        pct: (json['pct'] as num?)?.toDouble() ?? 0,
        funds: (json['funds'] as num?)?.toInt() ?? 0,
        total: (json['total'] as num?)?.toInt() ?? 0,
      );

  final DateTime date;
  final double pct;
  final int funds;
  final int total;
}

/// Relative percentage change of fund presence over each lookback window.
/// A field is null when there is no baseline snapshot old enough to compare
/// against (e.g. yearly is null until ≥1y of history exists).
class PresenceChanges {
  PresenceChanges({this.daily, this.weekly, this.monthly, this.yearly});

  factory PresenceChanges.fromJson(Map<String, dynamic> json) =>
      PresenceChanges(
        daily: (json['daily'] as num?)?.toDouble(),
        weekly: (json['weekly'] as num?)?.toDouble(),
        monthly: (json['monthly'] as num?)?.toDouble(),
        yearly: (json['yearly'] as num?)?.toDouble(),
      );

  final double? daily;
  final double? weekly;
  final double? monthly;
  final double? yearly;
}

/// A stock's fund-presence time series plus its d/w/m/y changes, returned by
/// GET /api/v1/entities/{id}/presence. Drives the real-time presence chart:
/// presence breadth (how many funds hold the stock) is the accumulation /
/// distribution signal behind buy/sell reads.
class PresenceSeries {
  PresenceSeries({
    required this.entity,
    required this.asOf,
    required this.presencePct,
    required this.funds,
    required this.total,
    required this.window,
    required this.changes,
    required this.points,
  });

  factory PresenceSeries.fromJson(Map<String, dynamic> json) => PresenceSeries(
        entity: json['entity'] as String? ?? '',
        asOf: json['as_of'] as String? ?? '',
        presencePct: (json['presence_pct'] as num?)?.toDouble() ?? 0,
        funds: (json['funds'] as num?)?.toInt() ?? 0,
        total: (json['total'] as num?)?.toInt() ?? 0,
        window: json['window'] as String? ?? '',
        changes: PresenceChanges.fromJson(
            (json['changes'] as Map<String, dynamic>?) ?? const {}),
        points: (json['points'] as List<dynamic>?)
                ?.map((p) => PresencePoint.fromJson(p as Map<String, dynamic>))
                .toList() ??
            const [],
      );

  final String entity;
  final String asOf; // latest snapshot date, "" when no data
  final double presencePct; // latest presence percentage
  final int funds; // latest distinct funds holding
  final int total; // latest distinct funds reporting
  final String window;
  final PresenceChanges changes;
  final List<PresencePoint> points;

  /// True when at least one snapshot is available to chart.
  bool get hasData => points.isNotEmpty;
}

// ─── Scraper monitoring ───────────────────────────────────────────────────────

class ScraperRun {
  ScraperRun({
    required this.runId,
    required this.startedAt,
    required this.status,
    required this.scanStart,
    required this.scanEnd,
    required this.indicesScanned,
    required this.disclosuresFound,
    required this.fundsTotal,
    required this.fundsDone,
    required this.holdingsSaved,
    required this.errors,
    required this.lastFund,
    required this.updatedAt,
    this.finishedAt,
  });

  factory ScraperRun.fromJson(Map<String, dynamic> j) => ScraperRun(
        runId: j['run_id'] as String,
        startedAt: DateTime.parse(j['started_at'] as String),
        finishedAt: j['finished_at'] != null
            ? DateTime.parse(j['finished_at'] as String)
            : null,
        status: j['status'] as String,
        scanStart: (j['scan_start'] as num).toInt(),
        scanEnd: (j['scan_end'] as num).toInt(),
        indicesScanned: (j['indices_scanned'] as num).toInt(),
        disclosuresFound: (j['disclosures_found'] as num).toInt(),
        fundsTotal: (j['funds_total'] as num).toInt(),
        fundsDone: (j['funds_done'] as num).toInt(),
        holdingsSaved: (j['holdings_saved'] as num).toInt(),
        errors: (j['errors'] as num).toInt(),
        lastFund: j['last_fund'] as String? ?? '',
        updatedAt: DateTime.parse(j['updated_at'] as String),
      );

  final String runId;
  final DateTime startedAt;
  final DateTime? finishedAt;
  final String status; // running | done | error
  final int scanStart;
  final int scanEnd;
  final int indicesScanned;
  final int disclosuresFound;
  final int fundsTotal;
  final int fundsDone;
  final int holdingsSaved;
  final int errors;
  final String lastFund;
  final DateTime updatedAt;

  bool get isRunning => status == 'running';
  double get fundsProgress => fundsTotal > 0 ? fundsDone / fundsTotal : 0.0;
}

class FundCoverage {
  FundCoverage({
    required this.totalSecurities,
    required this.stocksWithFunds,
    required this.totalHoldings,
    required this.totalFunds,
  });

  factory FundCoverage.fromJson(Map<String, dynamic> j) => FundCoverage(
        totalSecurities: (j['total_securities'] as num).toInt(),
        stocksWithFunds: (j['stocks_with_funds'] as num).toInt(),
        totalHoldings: (j['total_holdings'] as num).toInt(),
        totalFunds: (j['total_funds'] as num).toInt(),
      );

  final int totalSecurities;
  final int stocksWithFunds;
  final int totalHoldings;
  final int totalFunds;

  double get coverageRatio =>
      totalSecurities > 0 ? stocksWithFunds / totalSecurities : 0.0;
}

class ScraperStatus {
  ScraperStatus({required this.coverage, this.latestRun});

  factory ScraperStatus.fromJson(Map<String, dynamic> j) => ScraperStatus(
        latestRun: j['latest_run'] != null
            ? ScraperRun.fromJson(j['latest_run'] as Map<String, dynamic>)
            : null,
        coverage: FundCoverage.fromJson(j['coverage'] as Map<String, dynamic>),
      );

  final ScraperRun? latestRun;
  final FundCoverage coverage;
}

// ─── Live backend activity feed ───────────────────────────────────────────────

/// One real backend activity, derived from persisted DB writes (harvester price
/// fetches, analytic recomputes, scraper runs). Powers the "Canlı Olaylar" feed
/// in the monitoring panel — every entry reflects work the backend actually did.
class ActivityEvent {
  ActivityEvent({
    required this.ts,
    required this.source,
    required this.title,
    required this.detail,
    required this.count,
  });

  factory ActivityEvent.fromJson(Map<String, dynamic> j) => ActivityEvent(
        ts: DateTime.parse(j['ts'] as String),
        source: j['source'] as String? ?? '',
        title: j['title'] as String? ?? '',
        detail: j['detail'] as String? ?? '',
        count: (j['count'] as num?)?.toInt() ?? 0,
      );

  final DateTime ts;
  final String source; // harvester | analytic | scraper
  final String title;
  final String detail;
  final int count;
}

/// A persisted schema/DOM drift alert (drift_alert table), shown in the
/// dedicated drift section of the İzleme Paneli. A `breaking` alert means an
/// upstream source changed its structural contract (a required field removed or
/// a field retyped); it stays open — with field-level diffs — until the drift is
/// fixed and the alert is resolved.
class DriftAlert {
  DriftAlert({
    required this.id,
    required this.source,
    required this.severity,
    required this.summary,
    required this.added,
    required this.removed,
    required this.retyped,
    required this.status,
    required this.occurrences,
    required this.firstSeen,
    required this.lastSeen,
    this.resolvedAt,
  });

  factory DriftAlert.fromJson(Map<String, dynamic> j) => DriftAlert(
        id: (j['id'] as num?)?.toInt() ?? 0,
        source: j['source'] as String? ?? '',
        severity: j['severity'] as String? ?? '',
        summary: j['summary'] as String? ?? '',
        added: _strList(j['added']),
        removed: _strList(j['removed']),
        retyped: _strList(j['retyped']),
        status: j['status'] as String? ?? 'open',
        occurrences: (j['occurrences'] as num?)?.toInt() ?? 1,
        firstSeen: DateTime.parse(j['first_seen_at'] as String),
        lastSeen: DateTime.parse(j['last_seen_at'] as String),
        resolvedAt: j['resolved_at'] == null
            ? null
            : DateTime.tryParse(j['resolved_at'] as String),
      );

  final int id;
  final String source;
  final String severity; // benign | breaking
  final String summary;
  final List<String> added; // field paths added
  final List<String> removed; // field paths removed
  final List<String> retyped; // field paths whose kind changed
  final String status; // open | resolved
  final int occurrences;
  final DateTime firstSeen;
  final DateTime lastSeen;
  final DateTime? resolvedAt;

  bool get isBreaking => severity == 'breaking';
  bool get isOpen => status == 'open';
}

/// Parses a JSON value into a list of strings, tolerating null and non-list
/// shapes (returns an empty list rather than throwing).
List<String> _strList(dynamic v) {
  if (v is List) {
    return v.map((e) => e.toString()).toList();
  }
  return const <String>[];
}

/// Returns the open drift alerts, breaking-first then most-recently-seen, so the
/// most urgent outstanding contract break sits at the top of the panel. Pure and
/// independently testable.
List<DriftAlert> openDriftAlerts(List<DriftAlert> alerts) {
  final open = alerts.where((a) => a.isOpen).toList()
    ..sort((a, b) {
      if (a.isBreaking != b.isBreaking) {
        return a.isBreaking ? -1 : 1; // breaking first
      }
      return b.lastSeen.compareTo(a.lastSeen); // newest first
    });
  return open;
}
