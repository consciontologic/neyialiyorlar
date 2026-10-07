import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';

void main() {
  group('MetricInfo.displayName', () {
    test('uses name when the catalogue provides one', () {
      final m = MetricInfo.fromJson({
        'key': 'nimvi',
        'name': 'Net Inflow Momentum',
        'flag': 'fresh',
        'tier': 'daily',
      });

      expect(m.displayName, 'Net Inflow Momentum');
    });

    test('falls back to a prettified key when name is empty', () {
      final m = MetricInfo.fromJson({
        'key': 'velocity_accumulation',
        'name': '',
        'flag': 'approx',
        'tier': '',
      });

      expect(m.displayName, 'Velocity Accumulation');
    });

    test('prettifies a single-word key', () {
      final m = MetricInfo.fromJson({
        'key': 'nimvi',
        'name': '',
        'flag': 'approx',
        'tier': '',
      });

      expect(m.displayName, 'Nimvi');
    });
  });
}
