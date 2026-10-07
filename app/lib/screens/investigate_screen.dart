import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/api/client.dart';
import '../core/l10n/strings_tr.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';

/// Series windows offered by the explorer.
const List<String> kInvestigateWindows = ['7d', '30d', '90d'];

/// Investigate / Tweak console: inspect the running system, chart a metric
/// series, read the raw latest payload, and trigger a synthetic recompute.
///
/// This is the "investigate, use, test, tweak" surface for the local system.
class InvestigateScreen extends ConsumerStatefulWidget {
  const InvestigateScreen({super.key});

  @override
  ConsumerState<InvestigateScreen> createState() => _InvestigateScreenState();
}

class _InvestigateScreenState extends ConsumerState<InvestigateScreen> {
  String? _metricKey;
  String _entity = ''; // resolved from the live entitiesProvider on first build
  String _window = '30d';
  bool _ticking = false;

  Future<void> _generateTick() async {
    setState(() => _ticking = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final emitted = await ref.read(apiClientProvider).tick();
      // Refresh everything that just changed.
      ref.invalidate(adminStatusProvider);
      ref.invalidate(metricLatestProvider);
      ref.invalidate(metricSeriesProvider);
      messenger.showSnackBar(
        SnackBar(content: Text('Yeniden hesaplama $emitted değer üretti')),
      );
    } catch (e) {
      messenger.showSnackBar(
        SnackBar(content: Text('Veri üretme başarısız: $e')),
      );
    } finally {
      if (mounted) setState(() => _ticking = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final statusAsync = ref.watch(adminStatusProvider);
    final metricsAsync = ref.watch(metricsProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text(kScreenInvestigate),
        actions: [
          IconButton(
            tooltip: 'Durumu yenile',
            icon: const Icon(Icons.refresh),
            onPressed: () => ref.invalidate(adminStatusProvider),
          ),
        ],
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _statusCard(statusAsync),
          const SizedBox(height: 16),
          _explorerCard(metricsAsync),
          const SizedBox(height: 16),
          _tickCard(statusAsync),
        ],
      ),
    );
  }

  Widget _statusCard(AsyncValue<AdminStatus> statusAsync) => Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                kSectionSystemStatus,
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              const SizedBox(height: 12),
              statusAsync.when(
                data: (status) => Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _kv(kLabelService, status.service),
                    _kv(
                        kLabelSyntheticEngine,
                        status.syntheticEnabled
                            ? kLabelEnabled
                            : kLabelDisabled),
                    _kv(kLabelMetricsCatalog, '${status.metricsCatalog}'),
                    const SizedBox(height: 8),
                    Text(kLabelRowCounts,
                        style: TextStyle(fontWeight: FontWeight.w600)),
                    const SizedBox(height: 4),
                    ...status.counts.entries
                        .map((e) => _kv(e.key, '${e.value}')),
                    const SizedBox(height: 4),
                    Text(
                      'as of ${status.time.toLocal()}',
                      style:
                          TextStyle(fontSize: 11, color: Colors.grey.shade600),
                    ),
                  ],
                ),
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('$kLabelStatusUnavailable: $e'),
              ),
            ],
          ),
        ),
      );

  Widget _explorerCard(AsyncValue<List<MetricInfo>> metricsAsync) => Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                kSectionMetricExplorer,
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              const SizedBox(height: 12),
              metricsAsync.when(
                data: (metrics) {
                  if (metrics.isEmpty) {
                    return const Text('Katalogda henüz metrik yok.');
                  }
                  final selectedKey = _metricKey ?? metrics.first.key;
                  // Use live entities from the catalogue rather than a hardcoded list.
                  final entityList = ref.watch(entitiesProvider).valueOrNull ??
                      const <EntityInfo>[];
                  final entityIds = entityList.map((e) => e.id).toList();
                  // Ensure _entity is still valid after a reload.
                  final safeEntity = entityIds.contains(_entity)
                      ? _entity
                      : (entityIds.isNotEmpty ? entityIds.first : _entity);
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Wrap(
                        spacing: 12,
                        runSpacing: 8,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          _dropdown<String>(
                            label: kLabelMetric,
                            value: selectedKey,
                            items: {
                              for (final m in metrics)
                                m.key: metricNameTR(m.key,
                                    fallback: m.displayName),
                            },
                            onChanged: (v) => setState(() => _metricKey = v),
                          ),
                          _dropdown<String>(
                            label: kLabelEntity,
                            value: safeEntity,
                            items: {
                              for (final e in entityList) e.id: e.displayTicker,
                            },
                            onChanged: (v) =>
                                setState(() => _entity = v ?? safeEntity),
                          ),
                          _dropdown<String>(
                            label: kLabelWindow,
                            value: _window,
                            items: {
                              for (final w in kInvestigateWindows) w: w,
                            },
                            onChanged: (v) =>
                                setState(() => _window = v ?? _window),
                          ),
                        ],
                      ),
                      const SizedBox(height: 16),
                      _seriesView(selectedKey, safeEntity),
                      const SizedBox(height: 16),
                      _rawLatestView(selectedKey, safeEntity),
                    ],
                  );
                },
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('$kLabelMetricsUnavailable: $e'),
              ),
            ],
          ),
        ),
      );

  Widget _seriesView(String key, String entity) {
    final seriesAsync = ref.watch(metricSeriesProvider((key, entity, _window)));
    return seriesAsync.when(
      data: (points) {
        if (points.isEmpty) {
          return const Text('$kLabelSeriesPoints bulunamadı.');
        }
        final values = points.map((p) => p.value).toList();
        final latest = points.last;
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Text(
                  latest.value?.toStringAsFixed(4) ?? '—',
                  style: const TextStyle(
                      fontSize: 22, fontWeight: FontWeight.bold),
                ),
                const SizedBox(width: 8),
                Text(latest.flag.name,
                    style: TextStyle(color: Colors.grey.shade700)),
              ],
            ),
            const SizedBox(height: 8),
            SizedBox(
              height: 80,
              width: double.infinity,
              child: CustomPaint(
                painter: _SparklinePainter(values),
              ),
            ),
            const SizedBox(height: 4),
            Text('${points.length} $kLabelSeriesPoints · $_window',
                style: TextStyle(fontSize: 11, color: Colors.grey.shade600)),
          ],
        );
      },
      loading: () => const SizedBox(
        height: 80,
        child: Center(child: CircularProgressIndicator()),
      ),
      error: (e, _) => Text('Seri alınamadı: $e'),
    );
  }

  Widget _rawLatestView(String key, String entity) {
    final latestAsync = ref.watch(metricLatestProvider((key, entity)));
    return latestAsync.when(
      data: (value) {
        final pretty = value == null
            ? kLabelNoLatest
            : const JsonEncoder.withIndent('  ').convert(value.toJson());
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(kLabelRawLatest,
                style: TextStyle(fontWeight: FontWeight.w600)),
            const SizedBox(height: 4),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(8),
              decoration: BoxDecoration(
                color: Colors.black.withOpacity(0.04),
                borderRadius: BorderRadius.circular(4),
              ),
              child: SelectableText(
                pretty,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
              ),
            ),
          ],
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (e, _) => Text('Son değer alınamadı: $e'),
    );
  }

  Widget _tickCard(AsyncValue<AdminStatus> statusAsync) {
    final enabled = statusAsync.maybeWhen(
      data: (s) => s.syntheticEnabled,
      orElse: () => false,
    );
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(kSectionTweak,
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14)),
            const SizedBox(height: 8),
            Text(
              enabled ? kLabelTickEnabled : kLabelTickDisabled,
              style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
            ),
            const SizedBox(height: 12),
            FilledButton.icon(
              onPressed: (!enabled || _ticking) ? null : _generateTick,
              icon: _ticking
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.bolt),
              label: const Text(kLabelGenerateTick),
            ),
          ],
        ),
      ),
    );
  }

  Widget _kv(String k, String v) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 2),
        child: Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(k, style: TextStyle(color: Colors.grey.shade700)),
            Text(v, style: const TextStyle(fontWeight: FontWeight.w600)),
          ],
        ),
      );

  Widget _dropdown<T>({
    required String label,
    required T value,
    required Map<T, String> items,
    required ValueChanged<T?> onChanged,
  }) =>
      Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label,
              style: TextStyle(fontSize: 11, color: Colors.grey.shade600)),
          DropdownButton<T>(
            value: value,
            onChanged: onChanged,
            items: items.entries
                .map((e) => DropdownMenuItem<T>(
                      value: e.key,
                      child: Text(e.value),
                    ))
                .toList(),
          ),
        ],
      );
}

/// Minimal dependency-free sparkline. Nulls are treated as gaps (no segment).
class _SparklinePainter extends CustomPainter {
  _SparklinePainter(this.values);

  final List<double?> values;

  @override
  void paint(Canvas canvas, Size size) {
    final finite = values.whereType<double>().toList();
    if (finite.length < 2) return;

    var min = finite.first;
    var max = finite.first;
    for (final v in finite) {
      if (v < min) min = v;
      if (v > max) max = v;
    }
    final span = (max - min).abs() < 1e-9 ? 1.0 : (max - min);
    final dx =
        values.length > 1 ? size.width / (values.length - 1) : size.width;

    final paint = Paint()
      ..color = Colors.blue.shade600
      ..strokeWidth = 1.5
      ..style = PaintingStyle.stroke;

    Path? path;
    for (var i = 0; i < values.length; i++) {
      final v = values[i];
      if (v == null) {
        path = null; // gap: break the line
        continue;
      }
      final x = dx * i;
      final y = size.height - ((v - min) / span) * size.height;
      if (path == null) {
        path = Path()..moveTo(x, y);
      } else {
        path.lineTo(x, y);
      }
      // Draw incrementally so multiple segments (across gaps) all render.
      canvas.drawPath(path, paint);
    }
  }

  @override
  bool shouldRepaint(covariant _SparklinePainter oldDelegate) =>
      oldDelegate.values != values;
}
