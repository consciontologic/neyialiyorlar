import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n/strings_tr.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';

/// Real-time card showing a stock's *fund presence* — the share of all
/// reporting funds (TEFAS + KAP) that hold it — as a live sparkline plus
/// daily/weekly/monthly/yearly changes. Rising presence is accumulation
/// (more funds buying in), falling presence is distribution: the breadth
/// signal behind a buy/sell read.
///
/// Data comes from [presenceProvider], which auto-refreshes on a timer so the
/// chart stays current without a manual reload.
class PresenceChart extends ConsumerWidget {
  const PresenceChart({required this.entityId, super.key});

  final String entityId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final presenceAsync = ref.watch(presenceProvider(entityId));

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: presenceAsync.when(
          skipLoadingOnReload: true,
          skipLoadingOnRefresh: true,
          data: (series) =>
              series.hasData ? _content(context, series) : _empty(context),
          loading: () => const SizedBox(
            height: 120,
            child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
          ),
          error: (_, __) => _empty(context),
        ),
      ),
    );
  }

  // ── Populated state ──
  Widget _content(BuildContext context, PresenceSeries series) {
    final values = series.points.map((p) => p.pct).toList();
    final (lo, hi) = presenceBounds(values);
    final up = changeSign(series.changes.monthly ?? series.changes.weekly);
    final lineColor = up > 0
        ? Colors.green.shade600
        : up < 0
            ? Colors.red.shade600
            : Theme.of(context).colorScheme.primary;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _headerRow(context, series),
        const SizedBox(height: 12),
        // Current presence headline.
        Row(
          crossAxisAlignment: CrossAxisAlignment.baseline,
          textBaseline: TextBaseline.alphabetic,
          children: [
            Text(
              '%${series.presencePct.toStringAsFixed(1)}',
              style: const TextStyle(fontSize: 30, fontWeight: FontWeight.bold),
            ),
            const SizedBox(width: 8),
            Padding(
              padding: const EdgeInsets.only(bottom: 4),
              child: Text(
                '${series.funds} / ${series.total} $kLabelPresenceHolding',
                style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        // Live sparkline.
        SizedBox(
          height: 72,
          width: double.infinity,
          child: CustomPaint(
            painter: _PresenceSparklinePainter(
              values: values,
              lo: lo,
              hi: hi,
              lineColor: lineColor,
            ),
          ),
        ),
        const SizedBox(height: 12),
        // d / w / m / y change chips.
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            _changeChip(kLabelChangeDaily, series.changes.daily),
            _changeChip(kLabelChangeWeekly, series.changes.weekly),
            _changeChip(kLabelChangeMonthly, series.changes.monthly),
            _changeChip(kLabelChangeYearly, series.changes.yearly),
          ],
        ),
        const SizedBox(height: 10),
        // Plain-language breadth read (buy/sell framing).
        Row(
          children: [
            Icon(
              up > 0
                  ? Icons.trending_up
                  : up < 0
                      ? Icons.trending_down
                      : Icons.trending_flat,
              size: 16,
              color: lineColor,
            ),
            const SizedBox(width: 6),
            Expanded(
              child: Text(
                presenceReadTR(series.changes),
                style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _headerRow(BuildContext context, PresenceSeries series) => Row(
        children: [
          const Icon(Icons.groups_2_outlined, size: 18),
          const SizedBox(width: 6),
          const Text(
            kSectionFundPresence,
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
          ),
          const SizedBox(width: 8),
          // Live indicator dot.
          Container(
            width: 7,
            height: 7,
            decoration: BoxDecoration(
              color: Colors.green.shade500,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: 4),
          Text(
            kLabelPresenceLive,
            style: TextStyle(fontSize: 10, color: Colors.green.shade700),
          ),
          const Spacer(),
          if (series.asOf.isNotEmpty)
            Text(
              '$kLabelPresenceAsOf: ${series.asOf}',
              style: TextStyle(fontSize: 10, color: Colors.grey.shade500),
            ),
        ],
      );

  Widget _changeChip(String label, double? value) {
    final sign = changeSign(value);
    final color = sign > 0
        ? Colors.green.shade700
        : sign < 0
            ? Colors.red.shade700
            : Colors.grey.shade500;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: color.withOpacity(0.10),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(color: color.withOpacity(0.30)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(label,
              style: TextStyle(fontSize: 10, color: Colors.grey.shade600)),
          const SizedBox(height: 2),
          Text(
            formatSignedPct(value),
            style: TextStyle(
                fontSize: 13, fontWeight: FontWeight.bold, color: color),
          ),
        ],
      ),
    );
  }

  // ── Empty state ──
  Widget _empty(BuildContext context) => Row(
        children: [
          Icon(Icons.groups_2_outlined, size: 18, color: Colors.grey.shade500),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  kSectionFundPresence,
                  style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                ),
                const SizedBox(height: 4),
                Text(
                  kLabelPresenceEmpty,
                  style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
                ),
              ],
            ),
          ),
        ],
      );
}

// ─── Pure helpers (unit-tested in test/core/presence_test.dart) ───────────────

/// Formats a relative percentage change for a chip: '—' when null, else a
/// signed value with one decimal, e.g. '+12.3%', '-4.1%', '0.0%'.
String formatSignedPct(double? v) {
  if (v == null) {
    return '—';
  }
  final s = v.toStringAsFixed(1);
  return v > 0 ? '+$s%' : '$s%';
}

/// Sign of a change: 1 up, -1 down, 0 flat/unknown. Drives colour + icon.
int changeSign(double? v) {
  if (v == null || v == 0) {
    return 0;
  }
  return v > 0 ? 1 : -1;
}

/// Y-axis bounds for the sparkline with ~10% padding. Handles empty, single,
/// and all-equal series without collapsing to a zero-height range. Lower bound
/// is clamped at 0 since presence is a non-negative percentage.
(double, double) presenceBounds(List<double> values) {
  if (values.isEmpty) {
    return (0, 1);
  }
  var lo = values.first;
  var hi = values.first;
  for (final v in values) {
    if (v < lo) {
      lo = v;
    }
    if (v > hi) {
      hi = v;
    }
  }
  final double pad;
  if (hi <= lo) {
    pad = hi.abs() < 1 ? 1.0 : hi.abs() * 0.1;
  } else {
    pad = (hi - lo) * 0.1;
  }
  final low = lo - pad;
  return (low < 0 ? 0 : low, hi + pad);
}

/// A short Turkish breadth read for the buy/sell framing, from the monthly
/// change (falling back to weekly). Rising fund presence = accumulation;
/// falling = distribution.
String presenceReadTR(PresenceChanges c) {
  final m = c.monthly ?? c.weekly;
  if (m == null) {
    return 'Yeterli geçmiş veri yok';
  }
  if (m >= 5) {
    return 'Fon ilgisi belirgin artıyor — birikim sinyali';
  }
  if (m > 0) {
    return 'Fon ilgisi hafif artıyor';
  }
  if (m <= -5) {
    return 'Fon ilgisi belirgin azalıyor — dağıtım sinyali';
  }
  if (m < 0) {
    return 'Fon ilgisi hafif azalıyor';
  }
  return 'Fon ilgisi yatay';
}

/// Maps presence values to screen offsets within [size] given [lo]/[hi] bounds.
/// Exposed for testing the sparkline geometry. A single point is centred; an
/// empty series yields no offsets.
List<Offset> presenceOffsets(
    List<double> values, Size size, double lo, double hi) {
  if (values.isEmpty) {
    return const [];
  }
  final span = hi - lo;
  double y(double v) {
    if (span <= 0) {
      return size.height / 2;
    }
    return size.height - ((v - lo) / span) * size.height;
  }

  if (values.length == 1) {
    return [Offset(size.width / 2, y(values.first))];
  }
  final step = size.width / (values.length - 1);
  return [
    for (var i = 0; i < values.length; i++) Offset(i * step, y(values[i])),
  ];
}

class _PresenceSparklinePainter extends CustomPainter {
  _PresenceSparklinePainter({
    required this.values,
    required this.lo,
    required this.hi,
    required this.lineColor,
  });

  final List<double> values;
  final double lo;
  final double hi;
  final Color lineColor;

  @override
  void paint(Canvas canvas, Size size) {
    final offsets = presenceOffsets(values, size, lo, hi);
    if (offsets.isEmpty) {
      return;
    }

    // Single point: just a dot.
    if (offsets.length == 1) {
      canvas.drawCircle(offsets.first, 3, Paint()..color = lineColor);
      return;
    }

    final line = Path()..moveTo(offsets.first.dx, offsets.first.dy);
    for (var i = 1; i < offsets.length; i++) {
      line.lineTo(offsets[i].dx, offsets[i].dy);
    }

    // Filled area under the line.
    final area = Path.from(line)
      ..lineTo(offsets.last.dx, size.height)
      ..lineTo(offsets.first.dx, size.height)
      ..close();
    final areaPaint = Paint()
      ..style = PaintingStyle.fill
      ..color = lineColor.withOpacity(0.12);
    final linePaint = Paint()
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2
      ..strokeCap = StrokeCap.round
      ..strokeJoin = StrokeJoin.round
      ..color = lineColor;

    canvas
      ..drawPath(area, areaPaint)
      ..drawPath(line, linePaint)
      // Emphasise the latest point.
      ..drawCircle(offsets.last, 3, Paint()..color = lineColor);
  }

  @override
  bool shouldRepaint(_PresenceSparklinePainter old) =>
      old.values != values ||
      old.lo != lo ||
      old.hi != hi ||
      old.lineColor != lineColor;
}
