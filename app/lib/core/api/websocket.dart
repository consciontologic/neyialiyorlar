import 'dart:async';
import 'package:web_socket_channel/web_socket_channel.dart';

/// WebSocket channel wrapper with reconnect logic
class MetricsWebSocketChannel {
  MetricsWebSocketChannel({
    String? url,
    this.minBackoffMs = const Duration(milliseconds: 100),
    this.maxBackoffMs = const Duration(seconds: 30),
  }) {
    _streamController = StreamController.broadcast();
    this.url = url ?? _getDefaultWebSocketUrl();
  }
  late final String url;
  final Duration minBackoffMs;
  final Duration maxBackoffMs;
  WebSocketChannel? _channel;
  late StreamController<dynamic> _streamController;
  int _reconnectAttempt = 0;

  /// Connect to the WebSocket server
  Future<void> connect() async {
    try {
      _channel = WebSocketChannel.connect(Uri.parse(url));
      _reconnectAttempt = 0;

      // Listen to incoming messages
      _channel!.stream.listen(
        (message) {
          _streamController.add(message);
        },
        onError: (Object error) {
          _streamController.addError(error);
          _reconnect();
        },
        onDone: _reconnect,
      );
    } catch (e) {
      _streamController.addError(e);
      _reconnect();
    }
  }

  /// Reconnect with exponential backoff
  Future<void> _reconnect() async {
    final backoffMs = _calculateBackoff();
    await Future<void>.delayed(Duration(milliseconds: backoffMs));
    _reconnectAttempt++;
    await connect();
  }

  /// Calculate exponential backoff with jitter
  int _calculateBackoff() {
    final baseMs = minBackoffMs.inMilliseconds;
    final maxMs = maxBackoffMs.inMilliseconds;
    final exponential = baseMs * (1 << (_reconnectAttempt.clamp(0, 5)));
    final withJitter = exponential + (exponential ~/ 4); // Add up to 25% jitter
    return withJitter.clamp(baseMs, maxMs);
  }

  /// Get the stream of metric updates
  Stream<dynamic> get stream => _streamController.stream;

  /// Send a message
  void send(dynamic data) {
    _channel?.sink.add(data);
  }

  /// Close the connection
  Future<void> close() async {
    await _channel?.sink.close();
    await _streamController.close();
  }

  /// Check if connected
  bool get isConnected => _channel != null;
}

/// Helper to construct the WebSocket URL from the current page location.
String _getDefaultWebSocketUrl() => defaultWebSocketUrl(Uri.base);

/// Builds the metrics WebSocket URL from a base URI (the current page on web).
///
/// It mirrors the page scheme — `https`/`wss` -> `wss`, otherwise `ws` — so the
/// connection works behind the nginx TLS terminator without tripping the
/// browser's mixed-content block, and reuses the page host so it always targets
/// the same origin that served the app.
String defaultWebSocketUrl(Uri base) {
  final secure = base.scheme == 'https' || base.scheme == 'wss';
  final scheme = secure ? 'wss' : 'ws';
  final host =
      base.authority.isNotEmpty ? base.authority : 'neyialiyorlar.local';
  return '$scheme://$host/api/v1/ws';
}
