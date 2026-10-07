import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';
import 'package:neyialiyorlar/core/state/providers.dart';

EntityInfo _stock(String id, int funds, {String type = 'security'}) =>
    EntityInfo(id: id, type: type, displayTicker: id, fundCount: funds);

void main() {
  group('mostHeldStocks', () {
    test('orders securities by fundCount descending', () {
      final out = mostHeldStocks([
        _stock('AAA', 3),
        _stock('BBB', 10),
        _stock('CCC', 7),
      ]);

      expect(out.map((e) => e.id).toList(), ['BBB', 'CCC', 'AAA']);
    });

    test('excludes securities held by no fund', () {
      final out = mostHeldStocks([
        _stock('AAA', 0),
        _stock('BBB', 5),
        _stock('CCC', 0),
      ]);

      expect(out.map((e) => e.id).toList(), ['BBB']);
    });

    test('excludes non-security entities (baskets/funds)', () {
      final out = mostHeldStocks([
        _stock('XU100', 99, type: 'basket'),
        _stock('THYAO', 12),
      ]);

      expect(out.map((e) => e.id).toList(), ['THYAO']);
    });

    test('breaks ties deterministically by id (ascending)', () {
      final out = mostHeldStocks([
        _stock('GARAN', 8),
        _stock('AKBNK', 8),
        _stock('THYAO', 8),
      ]);

      // Equal fundCount -> stable, alphabetical by id.
      expect(out.map((e) => e.id).toList(), ['AKBNK', 'GARAN', 'THYAO']);
    });

    test('respects the limit', () {
      final out = mostHeldStocks([
        _stock('AAA', 5),
        _stock('BBB', 4),
        _stock('CCC', 3),
        _stock('DDD', 2),
      ], limit: 2);

      expect(out.map((e) => e.id).toList(), ['AAA', 'BBB']);
    });

    test('limit <= 0 returns all held stocks', () {
      final out = mostHeldStocks([
        _stock('AAA', 5),
        _stock('BBB', 4),
      ], limit: 0);

      expect(out.length, 2);
    });

    test('returns an empty list when nothing is held', () {
      expect(mostHeldStocks([_stock('AAA', 0)]), isEmpty);
      expect(mostHeldStocks(const []), isEmpty);
    });
  });
}
