import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n/strings_tr.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';
import '../widgets/metric_value_card.dart';
import '../widgets/presence_chart.dart';

/// Per-stock detail screen: shows entity header, buy/sell signal, index/fund
/// memberships, and live metric cards.
class StockDetailScreen extends ConsumerWidget {
  const StockDetailScreen({required this.entity, super.key});

  final EntityInfo entity;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final metricsAsync = ref.watch(metricsProvider);
    final signalAsync =
        entity.isSecurity ? ref.watch(signalProvider(entity.id)) : null;
    final fundsAsync =
        entity.isSecurity ? ref.watch(entityFundsProvider(entity.id)) : null;

    // Which BIST index baskets include this entity.
    final basketMemberships = entity.baskets
        .where((id) => ['XU030', 'XU100', 'XU500', 'XUTUM'].contains(id))
        .toList();

    final inWatchlist = ref.watch(watchlistProvider).contains(entity.id);

    return Scaffold(
      appBar: AppBar(
        title: Text(entity.displayTicker),
        actions: [
          IconButton(
            tooltip: inWatchlist
                ? 'İzleme listesinden çıkar'
                : 'İzleme listesine ekle',
            icon: Icon(
              inWatchlist ? Icons.star_rounded : Icons.star_outline_rounded,
              color: inWatchlist ? Colors.amber.shade600 : null,
            ),
            onPressed: () =>
                ref.read(watchlistProvider.notifier).toggle(entity.id),
          ),
        ],
      ),
      body: metricsAsync.when(
        data: (metrics) => CustomScrollView(
          slivers: [
            SliverToBoxAdapter(
              child: _header(
                context,
                metrics,
                signalAsync,
                basketMemberships,
                fundsAsync,
              ),
            ),
            SliverPadding(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
              sliver: SliverGrid(
                gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                  maxCrossAxisExtent: 200,
                  childAspectRatio: 1.1,
                  mainAxisSpacing: 8,
                  crossAxisSpacing: 8,
                ),
                delegate: SliverChildBuilderDelegate(
                  (context, i) => MetricValueCard(
                    metricKey: metrics[i].key,
                    metricName: metrics[i].displayName,
                    entity: entity.id,
                  ),
                  childCount: metrics.length,
                ),
              ),
            ),
          ],
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Metrik yüklenemedi: $e')),
      ),
    );
  }

  Widget _header(
    BuildContext context,
    List<MetricInfo> metrics,
    AsyncValue<StockSignal>? signalAsync,
    List<String> basketIds,
    AsyncValue<List<FundHolding>>? fundsAsync,
  ) =>
      Padding(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Entity info card
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            entity.displayTicker,
                            style: const TextStyle(
                              fontSize: 24,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ),
                        Chip(
                          label: Text(entity.isSecurity ? 'Hisse' : 'Sepet'),
                          visualDensity: VisualDensity.compact,
                        ),
                      ],
                    ),
                    const SizedBox(height: 12),
                    // Index membership — always shown
                    _indexMembershipRow(basketIds),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 8),
            // Signal card (securities only)
            if (entity.isSecurity && signalAsync != null)
              _signalCard(context, signalAsync),
            // Real-time fund-presence chart (securities only)
            if (entity.isSecurity) ...[
              const SizedBox(height: 8),
              PresenceChart(entityId: entity.id),
            ],
            // Fund holdings section (securities only)
            if (entity.isSecurity && fundsAsync != null) ...[
              const SizedBox(height: 8),
              _fundsCard(context, fundsAsync),
            ],
            const SizedBox(height: 8),
          ],
        ),
      );

  // ── Signal card ─────────────────────────────────────────────────────────────
  Widget _signalCard(
          BuildContext context, AsyncValue<StockSignal> signalAsync) =>
      Card(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Row(
                children: [
                  Icon(Icons.analytics_outlined, size: 18),
                  SizedBox(width: 6),
                  Text(
                    'Al / Sat Sinyali',
                    style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
                  ),
                ],
              ),
              const SizedBox(height: 12),
              signalAsync.when(
                data: _signalWidget,
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (e, _) => Text('Sinyal hesaplanamadı: $e',
                    style: const TextStyle(color: Colors.red)),
              ),
            ],
          ),
        ),
      );

  Widget _signalWidget(StockSignal sig) {
    final (color, icon, label) = switch (sig.signal) {
      SignalDirection.BUY => (Colors.green.shade700, Icons.trending_up, 'AL'),
      SignalDirection.SELL => (Colors.red.shade700, Icons.trending_down, 'SAT'),
      SignalDirection.HOLD => (
          Colors.orange.shade700,
          Icons.trending_flat,
          'BEKLE'
        ),
      SignalDirection.INSUFFICIENT_DATA => (
          Colors.grey,
          Icons.help_outline,
          'YETERSİZ VERİ'
        ),
    };

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              decoration: BoxDecoration(
                color: color.withOpacity(0.12),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(color: color.withOpacity(0.4)),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(icon, color: color, size: 20),
                  const SizedBox(width: 6),
                  Text(
                    label,
                    style: TextStyle(
                        color: color,
                        fontWeight: FontWeight.bold,
                        fontSize: 16),
                  ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            if (sig.signal != SignalDirection.INSUFFICIENT_DATA) ...[
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('Fiyat: ${sig.price.toStringAsFixed(2)} ₺',
                      style: const TextStyle(fontSize: 12)),
                  Text(
                    'Sapma: ${sig.score >= 0 ? '+' : ''}${sig.score.toStringAsFixed(2)}%  •  '
                    '5 günlük ort: ${sig.ma5.toStringAsFixed(2)}  /  '
                    '20 günlük ort: ${sig.ma20.toStringAsFixed(2)}',
                    style: const TextStyle(fontSize: 12),
                  ),
                ],
              ),
            ],
          ],
        ),
        const SizedBox(height: 8),
        Text(
          sig.reason,
          style: TextStyle(fontSize: 12, color: Colors.grey.shade700),
        ),
        const SizedBox(height: 4),
        Text(
          'Güven: ${sig.confidence.name}  •  ${_formatAge(sig.computedAt)}',
          style: TextStyle(fontSize: 11, color: Colors.grey.shade500),
        ),
      ],
    );
  }

  // ── Index membership row (always shown inside the info card) ────────────────
  Widget _indexMembershipRow(List<String> basketIds) => Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          const Icon(Icons.bar_chart, size: 16, color: Colors.grey),
          const SizedBox(width: 6),
          Text(
            'Endeks: ',
            style: TextStyle(fontSize: 13, color: Colors.grey.shade600),
          ),
          if (basketIds.isEmpty)
            Text(
              'Endeks dışı',
              style: TextStyle(fontSize: 13, color: Colors.grey.shade500),
            )
          else
            Wrap(
              spacing: 6,
              children: basketIds.map((id) {
                final label = _basketLabel(id);
                final color = id == 'XU030'
                    ? Colors.blue
                    : id == 'XU100'
                        ? Colors.indigo
                        : id == 'XU500'
                            ? Colors.teal
                            : Colors.blueGrey;
                return Chip(
                  label: Text(label, style: const TextStyle(fontSize: 11)),
                  backgroundColor: color.withOpacity(0.12),
                  side: BorderSide(color: color.withOpacity(0.35)),
                  visualDensity: VisualDensity.compact,
                  padding: EdgeInsets.zero,
                );
              }).toList(),
            ),
        ],
      );

  // ── Fund holdings card ────────────────────────────────────────────────────
  Widget _fundsCard(
    BuildContext context,
    AsyncValue<List<FundHolding>> fundsAsync,
  ) =>
      fundsAsync.when(
        data: (funds) {
          if (funds.isEmpty) {
            // No data yet — render nothing rather than an empty state card.
            return const SizedBox.shrink();
          }
          return Card(
            child: ExpansionTile(
              leading:
                  const Icon(Icons.account_balance_wallet_outlined, size: 20),
              title: Text(
                kSectionFundHoldings,
                style:
                    const TextStyle(fontWeight: FontWeight.bold, fontSize: 14),
              ),
              subtitle: Text(
                '${funds.length} fon portföyünde yer alıyor',
                style: TextStyle(fontSize: 11, color: Colors.grey.shade600),
              ),
              initiallyExpanded: true,
              children: funds.map((f) {
                final hasWeight = f.weightPct != null;
                final sourceColor = f.source == 'kap'
                    ? Colors.green.shade700
                    : Colors.blue.shade700;
                return ListTile(
                  dense: true,
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 16, vertical: 2),
                  title: Text(
                    f.fundCode,
                    style: const TextStyle(
                        fontWeight: FontWeight.w600, fontSize: 13),
                  ),
                  subtitle: Text(
                    f.fundTitle,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(fontSize: 11, color: Colors.grey.shade600),
                  ),
                  trailing: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    crossAxisAlignment: CrossAxisAlignment.end,
                    children: [
                      if (hasWeight)
                        Text(
                          '%${f.weightPct!.toStringAsFixed(2)}',
                          style: TextStyle(
                            fontSize: 13,
                            fontWeight: FontWeight.bold,
                            color: sourceColor,
                          ),
                        ),
                      Chip(
                        label: Text(
                          f.source.toUpperCase(),
                          style: const TextStyle(fontSize: 9),
                        ),
                        backgroundColor: sourceColor.withOpacity(0.1),
                        side: BorderSide(color: sourceColor.withOpacity(0.3)),
                        visualDensity: VisualDensity.compact,
                        padding: EdgeInsets.zero,
                        materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
                      ),
                    ],
                  ),
                );
              }).toList(),
            ),
          );
        },
        loading: () => const Padding(
          padding: EdgeInsets.symmetric(vertical: 12),
          child: Center(
              child: SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2))),
        ),
        error: (_, __) => const SizedBox.shrink(),
      );

  String _basketLabel(String id) => switch (id) {
        'XU030' => 'BIST30',
        'XU100' => 'BIST100',
        'XU500' => 'BIST500',
        'XUTUM' => 'BIST ALL',
        _ => id,
      };

  String _formatAge(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inMinutes < 60) return '${diff.inMinutes}dk önce';
    if (diff.inHours < 24) return '${diff.inHours}s önce';
    return '${diff.inDays}g önce';
  }
}
