import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/api/client.dart';

void main() {
  group('AdminStatus.fromJson', () {
    test('parses a full status payload', () {
      final status = AdminStatus.fromJson({
        'service': 'api',
        'time': '2025-06-23T10:00:00Z',
        'synthetic_enabled': true,
        'metrics_catalog': 10,
        'counts': {'metric_value': 5400, 'entity_ref': 5, 'source_health': 4},
      });

      expect(status.service, 'api');
      expect(status.syntheticEnabled, isTrue);
      expect(status.metricsCatalog, 10);
      expect(status.counts['metric_value'], 5400);
      expect(status.counts['entity_ref'], 5);
      expect(status.time, DateTime.utc(2025, 6, 23, 10));
    });

    test('tolerates missing optional fields', () {
      final status = AdminStatus.fromJson({
        'time': '2025-06-23T10:00:00Z',
      });

      expect(status.service, 'api');
      expect(status.syntheticEnabled, isFalse);
      expect(status.metricsCatalog, 0);
      expect(status.counts, isEmpty);
    });
  });
}
