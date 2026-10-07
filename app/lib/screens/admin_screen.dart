import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/api/client.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';

/// Monitoring panel: scraper progress, fund coverage, and DB row counts.
class AdminScreen extends ConsumerStatefulWidget {
  const AdminScreen({super.key});

  @override
  ConsumerState<AdminScreen> createState() => _AdminScreenState();
}

class _AdminScreenState extends ConsumerState<AdminScreen> {
  Timer? _ticker;

  @override
  void initState() {
    super.initState();
    // Poll every 5 s — keeps the scraper, counts, drift alerts, and live event
    // feed current.
    _ticker = Timer.periodic(const Duration(seconds: 5), (_) {
      ref.invalidate(scraperStatusProvider);
      ref.invalidate(adminStatusProvider);
      ref.invalidate(driftAlertsProvider);
      ref.invalidate(adminEventsProvider);
    });
  }

  @override
  void dispose() {
    _ticker?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scraperAsync = ref.watch(scraperStatusProvider);
    final adminAsync = ref.watch(adminStatusProvider);
    final driftAsync = ref.watch(driftAlertsProvider);
    final eventsAsync = ref.watch(adminEventsProvider);
    final cs = Theme.of(context).colorScheme;

    return Scaffold(
      appBar: AppBar(
        title: const Text('İzleme Paneli',
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 17)),
        centerTitle: false,
        actions: [
          IconButton(
            tooltip: 'Yenile',
            icon: const Icon(Icons.refresh),
            onPressed: () {
              ref.invalidate(scraperStatusProvider);
              ref.invalidate(adminStatusProvider);
              ref.invalidate(driftAlertsProvider);
              ref.invalidate(adminEventsProvider);
            },
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(scraperStatusProvider);
          ref.invalidate(adminStatusProvider);
          ref.invalidate(driftAlertsProvider);
          ref.invalidate(adminEventsProvider);
        },
        child: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            // ── Scraper progress card ─────────────────────────────────────
            scraperAsync.when(
              data: (s) => _ScraperCard(status: s),
              loading: () => const _LoadingCard(label: 'Tarayıcı Durumu'),
              error: (e, _) => _ErrorCard(label: 'Tarayıcı Durumu', error: e),
            ),
            const SizedBox(height: 12),

            // ── Schema-drift alerts (persist until resolved) ──────────────
            driftAsync.when(
              data: (alerts) => _DriftAlertsCard(alerts: alerts),
              loading: () =>
                  const _LoadingCard(label: 'Şema Kayması Uyarıları'),
              error: (e, _) =>
                  _ErrorCard(label: 'Şema Kayması Uyarıları', error: e),
            ),
            const SizedBox(height: 12),

            // ── DB row counts card ────────────────────────────────────────
            adminAsync.when(
              data: (s) => _DbCountsCard(counts: s.counts),
              loading: () => const _LoadingCard(label: 'Veritabanı Satırları'),
              error: (e, _) =>
                  _ErrorCard(label: 'Veritabanı Satırları', error: e),
            ),
            const SizedBox(height: 12),

            // ── Live backend activity feed ────────────────────────────────
            eventsAsync.when(
              data: (events) => _EventsCard(events: events),
              loading: () => const _LoadingCard(label: 'Canlı Olaylar'),
              error: (e, _) => _ErrorCard(label: 'Canlı Olaylar', error: e),
            ),
            const SizedBox(height: 12),

            // ── Polling indicator ─────────────────────────────────────────
            Center(
              child: Text(
                'Her 5 sn. otomatik yenilenir',
                style: TextStyle(
                    fontSize: 11, color: cs.onSurface.withOpacity(0.35)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ─── Scraper Progress Card ────────────────────────────────────────────────────

class _ScraperCard extends StatelessWidget {
  const _ScraperCard({required this.status});
  final ScraperStatus status;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final run = status.latestRun;
    final cov = status.coverage;

    return _Card(
      title: 'Fon Tarayıcı',
      icon: Icons.document_scanner_outlined,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Coverage summary
          _CoverageBar(
            label: 'Fon Verisi Olan Hisseler',
            value: cov.stocksWithFunds,
            total: cov.totalSecurities,
            color: cs.primary,
          ),
          const SizedBox(height: 4),
          Row(
            children: [
              _StatChip(label: 'Fon', value: '${cov.totalFunds}'),
              const SizedBox(width: 8),
              _StatChip(label: 'Pozisyon', value: '${cov.totalHoldings}'),
            ],
          ),
          if (run != null) ...[
            const Divider(height: 20),
            Row(
              children: [
                _StatusChip(status: run.status),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    run.runId,
                    style: TextStyle(
                        fontSize: 11,
                        color: cs.onSurface.withOpacity(0.45),
                        fontFamily: 'monospace'),
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            // Fund processing progress
            _CoverageBar(
              label: 'Fonlar İşlendi',
              value: run.fundsDone,
              total: run.fundsTotal > 0 ? run.fundsTotal : 1,
              color: run.isRunning ? cs.tertiary : cs.primary,
            ),
            const SizedBox(height: 4),
            Row(
              children: [
                _StatChip(
                    label: 'İndis Tarandı', value: _fmt(run.indicesScanned)),
                const SizedBox(width: 8),
                _StatChip(
                    label: 'Bildiri Bulundu', value: '${run.disclosuresFound}'),
                const SizedBox(width: 8),
                _StatChip(label: 'Hata', value: '${run.errors}'),
              ],
            ),
            if (run.lastFund.isNotEmpty) ...[
              const SizedBox(height: 6),
              Text(
                'Son fon: ${run.lastFund}',
                style: TextStyle(
                    fontSize: 11, color: cs.onSurface.withOpacity(0.5)),
              ),
            ],
            const SizedBox(height: 4),
            Text(
              'Güncellendi: ${_timeAgo(run.updatedAt)}',
              style: TextStyle(
                  fontSize: 10, color: cs.onSurface.withOpacity(0.35)),
            ),
          ] else ...[
            const Divider(height: 20),
            Text(
              'Henüz tarayıcı çalıştırılmadı.',
              style:
                  TextStyle(fontSize: 12, color: cs.onSurface.withOpacity(0.5)),
            ),
          ],
        ],
      ),
    );
  }
}

// ─── DB Counts Card ───────────────────────────────────────────────────────────

class _DbCountsCard extends StatelessWidget {
  const _DbCountsCard({required this.counts});
  final Map<String, int64> counts;

  @override
  Widget build(BuildContext context) => _Card(
        title: 'Veritabanı',
        icon: Icons.storage_outlined,
        child: Wrap(
          spacing: 8,
          runSpacing: 8,
          children: counts.entries
              .map((e) => _StatChip(
                    label: e.key,
                    value: _fmt(e.value.toInt()),
                  ))
              .toList(),
        ),
      );
}

// ─── Live Backend Activity Card ───────────────────────────────────────────────

class _EventsCard extends StatelessWidget {
  const _EventsCard({required this.events});
  final List<ActivityEvent> events;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return _Card(
      title: 'Canlı Olaylar',
      icon: Icons.bolt_outlined,
      child: events.isEmpty
          ? Text(
              'Henüz arka uç etkinliği yok. Hasat / analiz / tarama '
              'çalıştığında olaylar burada belirir.',
              style:
                  TextStyle(fontSize: 12, color: cs.onSurface.withOpacity(0.5)),
            )
          : Column(
              children: [
                for (final e in events) _EventRow(event: e),
              ],
            ),
    );
  }
}

class _EventRow extends StatelessWidget {
  const _EventRow({required this.event});
  final ActivityEvent event;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final (color, icon, label) = switch (event.source) {
      'harvester' => (Colors.green.shade600, Icons.download_outlined, 'Hasat'),
      'analytic' => (Colors.blue.shade600, Icons.functions, 'Analiz'),
      'scraper' => (
          Colors.orange.shade600,
          Icons.document_scanner_outlined,
          'Tarama'
        ),
      _ => (Colors.grey.shade500, Icons.circle_outlined, event.source),
    };
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.only(top: 1),
            child: Icon(icon, size: 15, color: color),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 5, vertical: 1),
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(4),
                        color: color.withOpacity(0.12),
                      ),
                      child: Text(label,
                          style: TextStyle(
                              fontSize: 9,
                              fontWeight: FontWeight.w700,
                              color: color)),
                    ),
                    const SizedBox(width: 6),
                    Expanded(
                      child: Text(event.title,
                          style: const TextStyle(
                              fontSize: 12.5, fontWeight: FontWeight.w600),
                          overflow: TextOverflow.ellipsis),
                    ),
                    Text(_timeAgo(event.ts),
                        style: TextStyle(
                            fontSize: 10,
                            color: cs.onSurface.withOpacity(0.4))),
                  ],
                ),
                if (event.detail.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.only(top: 1),
                    child: Text(event.detail,
                        style: TextStyle(
                            fontSize: 11,
                            color: cs.onSurface.withOpacity(0.6))),
                  ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Schema-Drift Alerts ──────────────────────────────────────────────────────

class _DriftAlertsCard extends StatelessWidget {
  const _DriftAlertsCard({required this.alerts});
  final List<DriftAlert> alerts;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return _Card(
      title: 'Şema Kayması Uyarıları',
      icon: Icons.warning_amber_rounded,
      child: alerts.isEmpty
          ? Text(
              'Şema kayması algılanmadı. Kaynak veri yapıları kararlı.',
              style:
                  TextStyle(fontSize: 12, color: cs.onSurface.withOpacity(0.5)),
            )
          : Column(
              children: [
                for (final a in alerts) _DriftAlertRow(alert: a),
              ],
            ),
    );
  }
}

class _DriftAlertRow extends ConsumerWidget {
  const _DriftAlertRow({required this.alert});
  final DriftAlert alert;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final cs = Theme.of(context).colorScheme;
    final accent =
        alert.isBreaking ? Colors.red.shade600 : Colors.orange.shade700;
    final badge = alert.isBreaking ? 'KIRILMA' : 'EK';

    return Container(
      margin: const EdgeInsets.symmetric(vertical: 5),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        color: accent.withOpacity(0.06),
        border: Border.all(color: accent.withOpacity(0.35)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 5, vertical: 1),
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(4),
                  color: accent.withOpacity(0.15),
                ),
                child: Text(badge,
                    style: TextStyle(
                        fontSize: 9,
                        fontWeight: FontWeight.w800,
                        color: accent)),
              ),
              const SizedBox(width: 6),
              Expanded(
                child: Text(alert.source,
                    style: const TextStyle(
                        fontSize: 12.5, fontWeight: FontWeight.w700),
                    overflow: TextOverflow.ellipsis),
              ),
              Text(_timeAgo(alert.lastSeen),
                  style: TextStyle(
                      fontSize: 10, color: cs.onSurface.withOpacity(0.4))),
            ],
          ),
          if (alert.summary.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(top: 4),
              child: Text(alert.summary,
                  style: TextStyle(
                      fontSize: 11.5, color: cs.onSurface.withOpacity(0.75))),
            ),
          // Field-level diffs.
          if (alert.retyped.isNotEmpty)
            _DiffLine(
                symbol: '~',
                color: Colors.orange.shade800,
                paths: alert.retyped),
          if (alert.removed.isNotEmpty)
            _DiffLine(
                symbol: '−', color: Colors.red.shade600, paths: alert.removed),
          if (alert.added.isNotEmpty)
            _DiffLine(
                symbol: '+', color: Colors.green.shade700, paths: alert.added),
          const SizedBox(height: 6),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('${alert.occurrences}× görüldü',
                  style: TextStyle(
                      fontSize: 10.5, color: cs.onSurface.withOpacity(0.45))),
              TextButton.icon(
                onPressed: () => _resolve(context, ref),
                icon: const Icon(Icons.check_circle_outline, size: 16),
                label: const Text('Çözüldü'),
                style: TextButton.styleFrom(
                  foregroundColor: Colors.green.shade700,
                  padding: const EdgeInsets.symmetric(horizontal: 8),
                  minimumSize: const Size(0, 32),
                  tapTargetSize: MaterialTapTargetSize.shrinkWrap,
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _resolve(BuildContext context, WidgetRef ref) async {
    final ok = await ref.read(apiClientProvider).resolveDriftAlert(alert.id);
    ref.invalidate(driftAlertsProvider);
    if (!context.mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(ok
            ? 'Uyarı çözüldü olarak işaretlendi'
            : 'Uyarı bulunamadı veya zaten çözülmüş'),
        duration: const Duration(seconds: 2),
      ),
    );
  }
}

/// One field-level diff line: a coloured symbol (+ added, − removed, ~ retyped)
/// followed by the affected field paths.
class _DiffLine extends StatelessWidget {
  const _DiffLine(
      {required this.symbol, required this.color, required this.paths});
  final String symbol;
  final Color color;
  final List<String> paths;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(symbol,
              style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w800,
                  color: color,
                  fontFamily: 'monospace')),
          const SizedBox(width: 6),
          Expanded(
            child: Text(paths.join(', '),
                style: TextStyle(
                    fontSize: 11.5, color: color, fontFamily: 'monospace')),
          ),
        ],
      ),
    );
  }
}

// ─── Shared widgets ───────────────────────────────────────────────────────────

class _Card extends StatelessWidget {
  const _Card({required this.title, required this.icon, required this.child});
  final String title;
  final IconData icon;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 16, color: cs.primary),
              const SizedBox(width: 6),
              Text(title,
                  style: const TextStyle(
                      fontWeight: FontWeight.bold, fontSize: 14)),
            ],
          ),
          const SizedBox(height: 12),
          child,
        ],
      ),
    );
  }
}

class _CoverageBar extends StatelessWidget {
  const _CoverageBar({
    required this.label,
    required this.value,
    required this.total,
    required this.color,
  });
  final String label;
  final int value;
  final int total;
  final Color color;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    final ratio = total > 0 ? (value / total).clamp(0.0, 1.0) : 0.0;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label,
                style: TextStyle(
                    fontSize: 12, color: cs.onSurface.withOpacity(0.65))),
            Text('$value / $total',
                style: TextStyle(
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                    color: cs.onSurface.withOpacity(0.8))),
          ],
        ),
        const SizedBox(height: 4),
        ClipRRect(
          borderRadius: BorderRadius.circular(3),
          child: LinearProgressIndicator(
            value: ratio,
            minHeight: 6,
            backgroundColor: color.withOpacity(0.1),
            valueColor: AlwaysStoppedAnimation<Color>(color),
          ),
        ),
      ],
    );
  }
}

class _StatChip extends StatelessWidget {
  const _StatChip({required this.label, required this.value});
  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(6),
        color: cs.surfaceContainerHighest,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(value,
              style:
                  const TextStyle(fontSize: 13, fontWeight: FontWeight.bold)),
          Text(label,
              style:
                  TextStyle(fontSize: 9, color: cs.onSurface.withOpacity(0.5))),
        ],
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});
  final String status;

  @override
  Widget build(BuildContext context) {
    final (color, label) = switch (status) {
      'running' => (Colors.orange.shade600, 'Çalışıyor'),
      'done' => (Colors.green.shade600, 'Tamamlandı'),
      'error' => (Colors.red.shade600, 'Hata'),
      _ => (Colors.grey.shade500, status),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(20),
        color: color.withOpacity(0.12),
        border: Border.all(color: color.withOpacity(0.4)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (status == 'running')
            Padding(
              padding: const EdgeInsets.only(right: 4),
              child: SizedBox(
                width: 8,
                height: 8,
                child: CircularProgressIndicator(
                    strokeWidth: 1.5,
                    valueColor: AlwaysStoppedAnimation<Color>(color)),
              ),
            ),
          Text(label,
              style: TextStyle(
                  fontSize: 11, fontWeight: FontWeight.w600, color: color)),
        ],
      ),
    );
  }
}

class _LoadingCard extends StatelessWidget {
  const _LoadingCard({required this.label});
  final String label;

  @override
  Widget build(BuildContext context) => _Card(
        title: label,
        icon: Icons.hourglass_empty_outlined,
        child: const Center(
          child: Padding(
            padding: EdgeInsets.symmetric(vertical: 8),
            child: CircularProgressIndicator(),
          ),
        ),
      );
}

class _ErrorCard extends StatelessWidget {
  const _ErrorCard({required this.label, required this.error});
  final String label;
  final Object error;

  @override
  Widget build(BuildContext context) => _Card(
        title: label,
        icon: Icons.error_outline,
        child: Text('Yüklenemedi: $error',
            style: TextStyle(
                fontSize: 12, color: Theme.of(context).colorScheme.error)),
      );
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

String _fmt(int n) {
  if (n >= 1000000) return '${(n / 1000000).toStringAsFixed(1)}M';
  if (n >= 1000) return '${(n / 1000).toStringAsFixed(1)}K';
  return '$n';
}

String _timeAgo(DateTime dt) {
  final diff = DateTime.now().difference(dt);
  if (diff.inSeconds < 60) return '${diff.inSeconds}sn önce';
  if (diff.inMinutes < 60) return '${diff.inMinutes}dk önce';
  return '${diff.inHours}sa önce';
}

// ignore: non_constant_identifier_names
typedef int64 = int;
