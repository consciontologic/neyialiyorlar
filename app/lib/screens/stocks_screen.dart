import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../core/l10n/strings_tr.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';
import '../widgets/flag_badge.dart';
import 'stock_detail_screen.dart';

/// Live list of monitored stocks (securities) plus the funds/baskets they roll
/// up into. A search bar at the top lets the user filter by ticker or ISIN.
/// Each row streams a headline metric over the WebSocket; tap a row to open the
/// full per-stock metric detail.
class StocksScreen extends ConsumerStatefulWidget {
  const StocksScreen({super.key});

  @override
  ConsumerState<StocksScreen> createState() => _StocksScreenState();
}

class _StocksScreenState extends ConsumerState<StocksScreen> {
  final _searchCtrl = TextEditingController();
  String _query = '';

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  bool _matches(EntityInfo e) {
    if (_query.isEmpty) {
      return true;
    }
    final q = _query.toLowerCase();
    return e.displayTicker.toLowerCase().contains(q) ||
        (e.isin?.toLowerCase().contains(q) ?? false) ||
        e.id.toLowerCase().contains(q);
  }

  @override
  Widget build(BuildContext context) {
    final entitiesAsync = ref.watch(entitiesProvider);
    final metricsAsync = ref.watch(metricsProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text(kScreenStocks),
        actions: [
          IconButton(
            tooltip: kLabelRefresh,
            icon: const Icon(Icons.refresh),
            onPressed: () {
              ref
                ..invalidate(entitiesProvider)
                ..invalidate(metricsProvider);
            },
          ),
        ],
      ),
      body: entitiesAsync.when(
        data: (entities) {
          final metrics = metricsAsync.valueOrNull ?? const <MetricInfo>[];
          final headline = metrics.isNotEmpty ? metrics.first : null;
          final stocks =
              entities.where((e) => e.isSecurity && _matches(e)).toList();

          return RefreshIndicator(
            onRefresh: () async {
              ref
                ..invalidate(entitiesProvider)
                ..invalidate(metricsProvider);
            },
            child: ListView(
              padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
              children: [
                // ── Arama çubuğu ──────────────────────────────────────────
                TextField(
                  controller: _searchCtrl,
                  decoration: InputDecoration(
                    hintText: kLabelSearch,
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: _query.isNotEmpty
                        ? IconButton(
                            icon: const Icon(Icons.clear),
                            onPressed: () {
                              _searchCtrl.clear();
                              setState(() => _query = '');
                            },
                          )
                        : null,
                    border: const OutlineInputBorder(),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 8),
                  ),
                  onChanged: (v) => setState(() => _query = v),
                ),
                const SizedBox(height: 12),
                _sectionLabel(kSectionStocks, stocks.length),
                if (stocks.isEmpty)
                  Padding(
                    padding: const EdgeInsets.all(24),
                    child: Center(child: Text(kLabelNoResults)),
                  )
                else
                  ...stocks.map(
                    (e) => _EntityTile(entity: e, headline: headline),
                  ),
              ],
            ),
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (e, _) => Center(child: Text('Hata: $e')),
      ),
    );
  }

  Widget _sectionLabel(String text, int count) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Text(
          '$text ($count)',
          style: const TextStyle(
            fontWeight: FontWeight.bold,
            fontSize: 13,
            letterSpacing: 0.3,
          ),
        ),
      );
}

/// A single tappable row for an entity, streaming one headline metric live.
class _EntityTile extends ConsumerWidget {
  const _EntityTile({
    required this.entity,
    required this.headline,
  });

  final EntityInfo entity;
  final MetricInfo? headline;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Card(
      margin: const EdgeInsets.symmetric(vertical: 4),
      child: ListTile(
        leading: CircleAvatar(
          backgroundColor:
              entity.isSecurity ? Colors.blue.shade100 : Colors.purple.shade100,
          child: Icon(
            entity.isSecurity ? Icons.show_chart : Icons.account_balance,
            color: entity.isSecurity
                ? Colors.blue.shade800
                : Colors.purple.shade800,
            size: 20,
          ),
        ),
        title: Text(
          entity.displayTicker,
          style: const TextStyle(fontWeight: FontWeight.bold),
        ),
        subtitle: _subtitle(),
        trailing: _trailing(ref),
        onTap: () => Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => StockDetailScreen(entity: entity),
          ),
        ),
      ),
    );
  }

  /// Subtitle row: a has-funds / no-funds badge followed by the ISIN.
  Widget _subtitle() => Padding(
        padding: const EdgeInsets.only(top: 4),
        child: Row(
          children: [
            _fundBadge(),
            if (entity.isin != null) ...[
              const SizedBox(width: 8),
              Flexible(
                child: Text(
                  entity.isin!,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(fontSize: 12, color: Colors.grey.shade600),
                ),
              ),
            ],
          ],
        ),
      );

  /// A pill that labels whether any fund holds this stock. Teal "{n} fon" when
  /// funds are known, muted "Fon yok" when none hold it yet.
  Widget _fundBadge() {
    final hasFunds = entity.hasFunds;
    final color = hasFunds ? Colors.teal : Colors.blueGrey;
    final label =
        hasFunds ? '${entity.fundCount} $kLabelFundsUnit' : kLabelNoFunds;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: color.withOpacity(0.12),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: color.withOpacity(0.4)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            hasFunds ? Icons.account_balance_wallet : Icons.money_off,
            size: 12,
            color: color.shade700,
          ),
          const SizedBox(width: 4),
          Text(
            label,
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: color.shade700,
            ),
          ),
        ],
      ),
    );
  }

  Widget _trailing(WidgetRef ref) {
    final inWatchlist = ref.watch(watchlistProvider).contains(entity.id);
    final starButton = GestureDetector(
      onTap: () => ref.read(watchlistProvider.notifier).toggle(entity.id),
      child: Padding(
        padding: const EdgeInsets.only(left: 4),
        child: Icon(
          inWatchlist ? Icons.star_rounded : Icons.star_outline_rounded,
          size: 20,
          color: inWatchlist ? Colors.amber.shade600 : Colors.grey.shade400,
        ),
      ),
    );

    final h = headline;
    if (h == null) {
      return Row(
        mainAxisSize: MainAxisSize.min,
        children: [starButton, const Icon(Icons.chevron_right)],
      );
    }

    final streamAsync = ref.watch(metricStreamProvider((h.key, entity.id)));
    final latestAsync = ref.watch(metricLatestProvider((h.key, entity.id)));
    final current = streamAsync.valueOrNull ?? latestAsync.valueOrNull;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Column(
          mainAxisAlignment: MainAxisAlignment.center,
          crossAxisAlignment: CrossAxisAlignment.end,
          children: [
            Text(
              current?.value?.toStringAsFixed(2) ?? '—',
              style: const TextStyle(
                fontWeight: FontWeight.bold,
                fontSize: 16,
              ),
            ),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 90),
              child: Text(
                metricNameTR(h.key, fallback: h.displayName),
                style: TextStyle(fontSize: 10, color: Colors.grey.shade600),
                overflow: TextOverflow.ellipsis,
                maxLines: 1,
              ),
            ),
          ],
        ),
        const SizedBox(width: 8),
        if (current != null)
          FlagBadge(flag: current.flag, size: 18)
        else
          const SizedBox(width: 18),
        starButton,
        const Icon(Icons.chevron_right),
      ],
    );
  }
}
