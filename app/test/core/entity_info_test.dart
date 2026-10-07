import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';

void main() {
  group('EntityInfo.fromJson', () {
    test('parses a security with a basket', () {
      final e = EntityInfo.fromJson({
        'id': 'GARAN',
        'type': 'security',
        'display_ticker': 'GARAN',
        'isin': 'TREGARA00017',
        'basket': 'banks',
      });

      expect(e.id, 'GARAN');
      expect(e.type, 'security');
      expect(e.displayTicker, 'GARAN');
      expect(e.isin, 'TREGARA00017');
      expect(e.basket, 'banks');
      expect(e.isSecurity, isTrue);
    });

    test('parses a standalone security (no basket)', () {
      final e = EntityInfo.fromJson({
        'id': 'THYAO',
        'type': 'security',
        'display_ticker': 'THYAO',
        'isin': 'TRETHY000019',
      });

      expect(e.basket, isNull);
      expect(e.isin, 'TRETHY000019');
      expect(e.isSecurity, isTrue);
    });

    test('parses a basket / fund entity', () {
      final e = EntityInfo.fromJson({
        'id': 'banks',
        'type': 'basket',
        'display_ticker': 'BANKS',
      });

      expect(e.type, 'basket');
      expect(e.displayTicker, 'BANKS');
      expect(e.isSecurity, isFalse);
      expect(e.basket, isNull);
      expect(e.isin, isNull);
    });

    test('applies defaults: type=security, display_ticker falls back to id',
        () {
      final e = EntityInfo.fromJson({'id': 'AKBNK'});

      expect(e.type, 'security');
      expect(e.displayTicker, 'AKBNK');
      expect(e.isSecurity, isTrue);
    });

    test('round-trips through toJson, omitting null optionals', () {
      final standalone = EntityInfo.fromJson({
        'id': 'THYAO',
        'type': 'security',
        'display_ticker': 'THYAO',
      });

      final json = standalone.toJson();
      expect(json.containsKey('isin'), isFalse);
      expect(json.containsKey('basket'), isFalse);
      expect(json['id'], 'THYAO');

      final reparsed = EntityInfo.fromJson(json);
      expect(reparsed.id, standalone.id);
      expect(reparsed.basket, isNull);
      expect(reparsed.isSecurity, isTrue);
    });

    test('parses fund_count and exposes hasFunds when funds hold the stock',
        () {
      final e = EntityInfo.fromJson({
        'id': 'THYAO',
        'type': 'security',
        'display_ticker': 'THYAO',
        'fund_count': 12,
      });

      expect(e.fundCount, 12);
      expect(e.hasFunds, isTrue);
    });

    test('defaults fund_count to 0 (no funds) when the field is absent', () {
      final e = EntityInfo.fromJson({
        'id': 'AKBNK',
        'type': 'security',
        'display_ticker': 'AKBNK',
      });

      expect(e.fundCount, 0);
      expect(e.hasFunds, isFalse);
    });

    test('round-trips fund_count through toJson', () {
      final held = EntityInfo.fromJson({
        'id': 'THYAO',
        'type': 'security',
        'display_ticker': 'THYAO',
        'fund_count': 3,
      });

      final reparsed = EntityInfo.fromJson(held.toJson());
      expect(reparsed.fundCount, 3);
      expect(reparsed.hasFunds, isTrue);
    });
  });
}
