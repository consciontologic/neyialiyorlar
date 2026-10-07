import 'dart:ui';

import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';
import 'package:neyialiyorlar/widgets/presence_chart.dart';

void main() {
  group('PresenceSeries.fromJson', () {
    test('parses full payload including null changes', () {
      final s = PresenceSeries.fromJson({
        'entity': 'THYAO',
        'as_of': '2026-06-26',
        'presence_pct': 33.3,
        'funds': 12,
        'total': 36,
        'window': '1y',
        'changes': {
          'daily': 1.2,
          'weekly': -0.5,
          'monthly': null,
          'yearly': 10.4,
        },
        'points': [
          {'date': '2026-06-01', 'pct': 30.0, 'funds': 10, 'total': 33},
          {'date': '2026-06-26', 'pct': 33.3, 'funds': 12, 'total': 36},
        ],
      });

      expect(s.entity, 'THYAO');
      expect(s.asOf, '2026-06-26');
      expect(s.presencePct, 33.3);
      expect(s.funds, 12);
      expect(s.total, 36);
      expect(s.points.length, 2);
      expect(s.points.first.date, DateTime.parse('2026-06-01'));
      expect(s.changes.daily, 1.2);
      expect(s.changes.weekly, -0.5);
      expect(s.changes.monthly, isNull);
      expect(s.changes.yearly, 10.4);
      expect(s.hasData, isTrue);
    });

    test('tolerates a sparse/empty payload', () {
      final s = PresenceSeries.fromJson({'entity': 'AAA'});
      expect(s.entity, 'AAA');
      expect(s.presencePct, 0);
      expect(s.points, isEmpty);
      expect(s.hasData, isFalse);
      expect(s.changes.daily, isNull);
      expect(s.changes.yearly, isNull);
    });
  });

  group('formatSignedPct', () {
    test('formats null, positive, negative and zero', () {
      expect(formatSignedPct(null), '—');
      expect(formatSignedPct(12.34), '+12.3%');
      expect(formatSignedPct(-4.06), '-4.1%');
      expect(formatSignedPct(0), '0.0%');
    });
  });

  group('changeSign', () {
    test('classifies direction', () {
      expect(changeSign(null), 0);
      expect(changeSign(0), 0);
      expect(changeSign(5), 1);
      expect(changeSign(-5), -1);
    });
  });

  group('presenceBounds', () {
    test('empty series gets a safe default', () {
      expect(presenceBounds(const []), (0.0, 1.0));
    });

    test('normal series pads ~10% and clamps lower bound at 0', () {
      final (lo, hi) = presenceBounds([10, 30]);
      expect(lo, closeTo(8, 0.001));
      expect(hi, closeTo(32, 0.001));

      final (lo2, _) = presenceBounds([0, 1]);
      expect(lo2, 0); // would be negative; clamped
    });

    test('flat series does not collapse to zero height', () {
      final (lo, hi) = presenceBounds([20, 20]);
      expect(hi - lo, greaterThan(0));
      expect(lo, lessThan(20));
      expect(hi, greaterThan(20));
    });
  });

  group('presenceReadTR', () {
    PresenceChanges ch({double? monthly, double? weekly}) =>
        PresenceChanges(monthly: monthly, weekly: weekly);

    test('null monthly+weekly is neutral', () {
      expect(presenceReadTR(ch()), 'Yeterli geçmiş veri yok');
    });
    test('strong rise is accumulation', () {
      expect(presenceReadTR(ch(monthly: 6)), contains('birikim'));
    });
    test('strong fall is distribution', () {
      expect(presenceReadTR(ch(monthly: -6)), contains('dağıtım'));
    });
    test('mild moves are reported softly', () {
      expect(presenceReadTR(ch(monthly: 2)), 'Fon ilgisi hafif artıyor');
      expect(presenceReadTR(ch(monthly: -2)), 'Fon ilgisi hafif azalıyor');
      expect(presenceReadTR(ch(monthly: 0)), 'Fon ilgisi yatay');
    });
    test('falls back to weekly when monthly is null', () {
      expect(presenceReadTR(ch(weekly: 6)), contains('birikim'));
    });
  });

  group('presenceOffsets', () {
    const size = Size(100, 50);

    test('empty series yields no offsets', () {
      expect(presenceOffsets(const [], size, 0, 1), isEmpty);
    });

    test('single point is centred horizontally', () {
      final out = presenceOffsets([10], size, 5, 15);
      expect(out.length, 1);
      expect(out.first.dx, 50);
      expect(out.first.dy, closeTo(25, 0.001)); // mid of padded range
    });

    test('two points span the full width and invert the y-axis', () {
      final out = presenceOffsets([0, 10], size, 0, 10);
      expect(out.length, 2);
      expect(out.first, const Offset(0, 50)); // low value -> bottom
      expect(out.last, const Offset(100, 0)); // high value -> top
    });

    test('flat range maps to the vertical centre', () {
      final out = presenceOffsets([7, 7], size, 7, 7);
      expect(out.every((o) => o.dy == 25), isTrue);
    });
  });
}
