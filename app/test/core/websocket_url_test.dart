import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/api/websocket.dart';

void main() {
  group('defaultWebSocketUrl', () {
    test('maps https page to wss (no mixed-content block)', () {
      final url =
          defaultWebSocketUrl(Uri.parse('https://neyialiyorlar.local/'));
      expect(url, 'wss://neyialiyorlar.local/api/v1/ws');
    });

    test('maps http page to ws', () {
      final url = defaultWebSocketUrl(Uri.parse('http://neyialiyorlar.local/'));
      expect(url, 'ws://neyialiyorlar.local/api/v1/ws');
    });

    test('reuses the page host and port (same origin)', () {
      final url = defaultWebSocketUrl(Uri.parse('http://localhost:5001/app'));
      expect(url, 'ws://localhost:5001/api/v1/ws');
    });

    test('falls back to neyialiyorlar.local when authority is absent', () {
      final url = defaultWebSocketUrl(Uri.parse('file:///tmp/index.html'));
      expect(url, 'ws://neyialiyorlar.local/api/v1/ws');
    });
  });

  group('MetricsWebSocketChannel.isConnected', () {
    // Regression: `_channel` was declared `late` with no initializer, so reading
    // `isConnected` before connect() threw a LateInitializationError. That sent
    // metricStreamProvider down its REST-only fallback and killed live WS updates.
    test('returns false before connect() without throwing', () {
      final ws = MetricsWebSocketChannel(url: 'ws://localhost/api/v1/ws');
      expect(ws.isConnected, isFalse);
    });
  });
}
