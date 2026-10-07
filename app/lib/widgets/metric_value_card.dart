import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n/strings_tr.dart';
import '../core/state/providers.dart';
import 'flag_badge.dart';

/// A single live metric tile for a `(metricKey, entity)` pair.
///
/// Subscribes to the WebSocket stream for real-time ticks and falls back to the
/// REST `latest` value while the stream warms up (or if the socket is down).
/// Hover or long-press shows a Tooltip with the Turkish metric description.
/// Wrapped in a [RepaintBoundary] so a tick only repaints this tile rather than
/// the surrounding grid.
class MetricValueCard extends ConsumerWidget {
  const MetricValueCard({
    required this.metricKey,
    required this.metricName,
    required this.entity,
    super.key,
  });

  final String metricKey;
  final String metricName;
  final String entity;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final streamAsync = ref.watch(metricStreamProvider((metricKey, entity)));
    final latestAsync = ref.watch(metricLatestProvider((metricKey, entity)));

    // Prefer the live stream value; fall back to the REST latest value.
    final current = streamAsync.valueOrNull ?? latestAsync.valueOrNull;
    // Only show the spinner while the REST fetch is in flight.
    // The stream provider can stay in "loading" indefinitely (e.g. when the
    // entity has no data and the stream completes without emitting), so we
    // must NOT include streamAsync.isLoading in the loading gate — otherwise
    // the spinner never clears for stocks without data.
    final loading = current == null && latestAsync.isLoading;

    final nameTR = metricNameTR(metricKey, fallback: metricName);
    final descTR = metricDescTR(metricKey);

    return RepaintBoundary(
      child: Tooltip(
        message: descTR ?? nameTR,
        preferBelow: true,
        waitDuration: const Duration(milliseconds: 400),
        child: Card(
          child: Padding(
            padding: const EdgeInsets.all(8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  nameTR,
                  style: const TextStyle(
                    fontWeight: FontWeight.bold,
                    fontSize: 12,
                  ),
                  overflow: TextOverflow.ellipsis,
                  maxLines: 2,
                ),
                const SizedBox(height: 8),
                Expanded(
                  child: Center(
                    child: loading
                        ? const SizedBox(
                            width: 18,
                            height: 18,
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : current == null
                            ? Text(
                                kLabelNoData,
                                style: const TextStyle(color: Colors.grey),
                              )
                            : Column(
                                mainAxisAlignment: MainAxisAlignment.center,
                                children: [
                                  Text(
                                    current.value?.toStringAsFixed(2) ?? '—',
                                    style: const TextStyle(
                                      fontSize: 18,
                                      fontWeight: FontWeight.bold,
                                    ),
                                  ),
                                  const SizedBox(height: 4),
                                  FlagBadge(flag: current.flag),
                                ],
                              ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
