import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';

void main() {
  group('DriftAlert.fromJson', () {
    test('parses a full payload including field-level diffs', () {
      final a = DriftAlert.fromJson({
        'id': 7,
        'source': 'yahoo',
        'severity': 'breaking',
        'summary': 'removed: chart.result[].indicators',
        'added': <String>[],
        'removed': ['chart.result[].indicators'],
        'retyped': ['chart.result[].timestamp'],
        'status': 'open',
        'occurrences': 3,
        'first_seen_at': '2026-06-20T10:00:00Z',
        'last_seen_at': '2026-06-26T12:00:00Z',
        'resolved_at': null,
      });

      expect(a.id, 7);
      expect(a.source, 'yahoo');
      expect(a.isBreaking, isTrue);
      expect(a.isOpen, isTrue);
      expect(a.removed, ['chart.result[].indicators']);
      expect(a.retyped, ['chart.result[].timestamp']);
      expect(a.added, isEmpty);
      expect(a.occurrences, 3);
      expect(a.resolvedAt, isNull);
    });

    test('tolerates missing optional fields with safe defaults', () {
      final a = DriftAlert.fromJson({
        'id': 1,
        'source': 'yahoo',
        'first_seen_at': '2026-06-26T12:00:00Z',
        'last_seen_at': '2026-06-26T12:00:00Z',
      });

      expect(a.severity, '');
      expect(a.summary, '');
      expect(a.added, isEmpty);
      expect(a.removed, isEmpty);
      expect(a.retyped, isEmpty);
      expect(a.status, 'open');
      expect(a.occurrences, 1);
      expect(a.isBreaking, isFalse);
      expect(a.isOpen, isTrue);
      expect(a.resolvedAt, isNull);
    });

    test('parses resolved_at when present', () {
      final a = DriftAlert.fromJson({
        'id': 2,
        'source': 'yahoo',
        'status': 'resolved',
        'first_seen_at': '2026-06-26T12:00:00Z',
        'last_seen_at': '2026-06-26T12:00:00Z',
        'resolved_at': '2026-06-26T13:00:00Z',
      });

      expect(a.isOpen, isFalse);
      expect(a.resolvedAt, isNotNull);
    });
  });

  group('openDriftAlerts', () {
    DriftAlert make(
            int id, String severity, String status, DateTime lastSeen) =>
        DriftAlert(
          id: id,
          source: 'yahoo',
          severity: severity,
          summary: '',
          added: const [],
          removed: const [],
          retyped: const [],
          status: status,
          occurrences: 1,
          firstSeen: lastSeen,
          lastSeen: lastSeen,
        );

    test('keeps only open alerts, breaking first then newest', () {
      final now = DateTime.utc(2026, 6, 26, 12);
      final out = openDriftAlerts([
        make(1, 'benign', 'open', now),
        make(2, 'breaking', 'open', now.subtract(const Duration(hours: 1))),
        make(3, 'breaking', 'open', now),
        make(4, 'benign', 'resolved', now), // filtered out
      ]);

      expect(out.map((a) => a.id).toList(), [3, 2, 1]);
    });

    test('returns empty when nothing is open', () {
      final now = DateTime.utc(2026, 6, 26, 12);
      final out = openDriftAlerts([
        make(1, 'breaking', 'resolved', now),
      ]);
      expect(out, isEmpty);
    });
  });
}
