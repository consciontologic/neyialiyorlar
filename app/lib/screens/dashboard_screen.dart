import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../core/l10n/strings_tr.dart';
import '../core/model/metric.dart';
import '../core/state/providers.dart';
import 'admin_screen.dart';
import 'stock_detail_screen.dart';
import 'stocks_screen.dart';

/// Main dashboard: live ticker strip, summary stats, top-mover rankings.
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      appBar: AppBar(
        title: const Text(kAppTitle,
            style: TextStyle(fontWeight: FontWeight.bold, fontSize: 17)),
        centerTitle: false,
        elevation: 0,
        actions: [
          IconButton(
            tooltip: 'İzleme Paneli',
            icon: const Icon(Icons.monitor_heart_outlined),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const AdminScreen()),
            ),
          ),
          IconButton(
            tooltip: kScreenStocks,
            icon: const Icon(Icons.format_list_bulleted),
            onPressed: () => Navigator.of(context).push(
              MaterialPageRoute<void>(builder: (_) => const StocksScreen()),
            ),
          ),
        ],
      ),
      body: Column(
        children: [
          const _LiveTickerStrip(),
          Expanded(
            child: RefreshIndicator(
              onRefresh: () async {
                ref.invalidate(entitiesProvider);
                ref.invalidate(topHeldProvider(50));
                ref.invalidate(topMoversProvider('nimvi'));
              },
              child: SingleChildScrollView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    _MarketSummaryRow(ref: ref),
                    const SizedBox(height: 20),
                    _WatchlistSection(ref: ref),
                    _TopHeldSection(ref: ref, limit: 50),
                    const SizedBox(height: 16),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Live Ticker Strip ────────────────────────────────────────────────────────

class _LiveTickerStrip extends ConsumerStatefulWidget {
  const _LiveTickerStrip();

  @override
  ConsumerState<_LiveTickerStrip> createState() => _LiveTickerStripState();
}

class _LiveTickerStripState extends ConsumerState<_LiveTickerStrip> {
  final _sc = ScrollController();
  Timer? _timer;

  @override
  void initState() {
    super.initState();
    _timer = Timer.periodic(const Duration(milliseconds: 2500), (_) {
      if (!_sc.hasClients) return;
      final max = _sc.position.maxScrollExtent;
      if (max == 0) return;
      final next = _sc.offset + 100;
      _sc.animateTo(
        next > max ? 0 : next,
        duration: const Duration(milliseconds: 600),
        curve: Curves.easeInOut,
      );
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    _sc.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final moversAsync = ref.watch(topMoversProvider('nimvi'));
    return Container(
      height: 34,
      color: const Color(0xFF0D1117),
      child: moversAsync.when(
        data: (movers) {
          if (movers.isEmpty) return const SizedBox.shrink();
          final items = [...movers, ...movers];
          return ListView.builder(
            controller: _sc,
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
            itemCount: items.length,
            itemBuilder: (_, i) {
              final m = items[i];
              final v = m.value ?? 50;
              final color = v >= 65
                  ? const Color(0xFF56D364)
                  : v >= 45
                      ? const Color(0xFFE3B341)
                      : const Color(0xFFFF7B72);
              final arrow = v >= 65
                  ? '▲'
                  : v <= 35
                      ? '▼'
                      : '●';
              return Padding(
                padding: const EdgeInsets.only(right: 24),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      m.entity,
                      style: const TextStyle(
                        color: Colors.white70,
                        fontSize: 11,
                        fontWeight: FontWeight.w600,
                        letterSpacing: 0.4,
                      ),
                    ),
                    const SizedBox(width: 4),
                    Text(
                      '${v.toStringAsFixed(1)} $arrow',
                      style: TextStyle(
                        color: color,
                        fontSize: 11,
                        fontWeight: FontWeight.bold,
                      ),
                    ),
                  ],
                ),
              );
            },
          );
        },
        loading: () => const Center(
          child: SizedBox(
            width: 14,
            height: 14,
            child: CircularProgressIndicator(
                strokeWidth: 1.5, color: Colors.white38),
          ),
        ),
        error: (_, __) => const SizedBox.shrink(),
      ),
    );
  }
}

// ─── Market Summary ───────────────────────────────────────────────────────────

class _MarketSummaryRow extends StatelessWidget {
  const _MarketSummaryRow({required this.ref});
  final WidgetRef ref;

  @override
  Widget build(BuildContext context) {
    final entities = ref.watch(entitiesProvider).valueOrNull ?? <EntityInfo>[];
    final securities = entities.where((e) => e.isSecurity).toList();
    final xu030 = securities.where((e) => e.baskets.contains('XU030')).length;

    return Row(
      children: [
        Expanded(
          child: _SummaryChip(
            value: '${securities.length}',
            label: kLabelTrackedStocks,
            icon: Icons.bar_chart_rounded,
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: _SummaryChip(
            value: '$xu030',
            label: kLabelXu030Count,
            icon: Icons.account_balance_outlined,
          ),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: _SummaryChip(
            value: kLabelDailyData,
            label: 'Güncelleme',
            icon: Icons.schedule_outlined,
          ),
        ),
      ],
    );
  }
}

class _SummaryChip extends StatelessWidget {
  const _SummaryChip({
    required this.value,
    required this.label,
    required this.icon,
  });
  final String value;
  final String label;
  final IconData icon;

  @override
  Widget build(BuildContext context) {
    final cs = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: cs.outlineVariant),
      ),
      child: Row(
        children: [
          Icon(icon, size: 16, color: cs.primary),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(value,
                    style: const TextStyle(
                        fontSize: 15, fontWeight: FontWeight.bold)),
                Text(label,
                    style: TextStyle(
                        fontSize: 10, color: cs.onSurface.withOpacity(0.5))),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

// ─── Watchlist Section ────────────────────────────────────────────────────────

class _WatchlistSection extends StatelessWidget {
  const _WatchlistSection({required this.ref});
  final WidgetRef ref;

  @override
  Widget build(BuildContext context) {
    final watchlist = ref.watch(watchlistProvider);
    final cs = Theme.of(context).colorScheme;
    final entityMap = {
      for (final e
          in (ref.watch(entitiesProvider).valueOrNull ?? <EntityInfo>[]))
        e.id: e
    };

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.star_rounded, size: 16, color: Colors.amber.shade600),
            const SizedBox(width: 6),
            const Text('İzleme Listesi',
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14)),
          ],
        ),
        const SizedBox(height: 8),
        if (watchlist.isEmpty)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: cs.outlineVariant),
            ),
            child: Row(
              children: [
                Icon(Icons.star_outline_rounded,
                    size: 14, color: cs.onSurface.withOpacity(0.4)),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    'Hisselerin yanındaki ★ simgesine dokunarak listeye ekleyin',
                    style: TextStyle(
                        fontSize: 12, color: cs.onSurface.withOpacity(0.5)),
                  ),
                ),
              ],
            ),
          )
        else
          Container(
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: cs.outlineVariant),
            ),
            child: ListView.separated(
              shrinkWrap: true,
              physics: const NeverScrollableScrollPhysics(),
              itemCount: watchlist.length,
              separatorBuilder: (_, __) =>
                  Divider(height: 1, color: cs.outlineVariant),
              itemBuilder: (ctx, i) {
                final id = watchlist.elementAt(i);
                final entity = entityMap[id] ??
                    EntityInfo(id: id, type: 'security', displayTicker: id);
                return ListTile(
                  dense: true,
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
                  leading: Icon(Icons.show_chart, size: 18, color: cs.primary),
                  title: Text(entity.displayTicker,
                      style: const TextStyle(
                          fontWeight: FontWeight.w600, fontSize: 13)),
                  trailing: GestureDetector(
                    onTap: () =>
                        ref.read(watchlistProvider.notifier).toggle(id),
                    child: Icon(Icons.star_rounded,
                        size: 20, color: Colors.amber.shade600),
                  ),
                  onTap: () => Navigator.of(ctx).push(
                    MaterialPageRoute<void>(
                      builder: (_) => StockDetailScreen(entity: entity),
                    ),
                  ),
                );
              },
            ),
          ),
        const SizedBox(height: 16),
      ],
    );
  }
}

// ─── Most Held By Funds ───────────────────────────────────────────────────────

/// Home page ranking of the securities held by the most funds.
///
/// Replaces the former daily buy/sell-velocity ranking: the list is ordered by
/// how many funds hold each stock (its `fundCount`), highest first, with the
/// holding funds shown beneath each ticker. Data comes from [topHeldProvider],
/// derived from the already-loaded entity catalogue (no extra request).
class _TopHeldSection extends StatelessWidget {
  const _TopHeldSection({required this.ref, required this.limit});
  final WidgetRef ref;
  final int limit;

  @override
  Widget build(BuildContext context) {
    final heldAsync = ref.watch(topHeldProvider(limit));
    final cs = Theme.of(context).colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(Icons.account_balance_wallet_outlined,
                size: 16, color: cs.primary),
            const SizedBox(width: 6),
            const Text(kSectionMostHeld,
                style: TextStyle(fontWeight: FontWeight.bold, fontSize: 14)),
          ],
        ),
        const SizedBox(height: 8),
        heldAsync.when(
          data: (stocks) {
            if (stocks.isEmpty) {
              return Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Text(kLabelMostHeldEmpty,
                    style: TextStyle(
                        fontSize: 12, color: cs.onSurface.withOpacity(0.5))),
              );
            }
            // Bars are scaled against the most-held stock at the top.
            final maxCount = stocks.first.fundCount;
            return Container(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(10),
                border: Border.all(color: cs.outlineVariant),
              ),
              child: ListView.separated(
                shrinkWrap: true,
                physics: const NeverScrollableScrollPhysics(),
                itemCount: stocks.length,
                separatorBuilder: (_, __) =>
                    Divider(height: 1, color: cs.outlineVariant),
                itemBuilder: (ctx, i) {
                  final e = stocks[i];
                  final ratio = maxCount > 0 ? e.fundCount / maxCount : 0.0;
                  final inWatchlist =
                      ref.watch(watchlistProvider).contains(e.id);

                  return ListTile(
                    dense: true,
                    contentPadding:
                        const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                    leading: Container(
                      width: 28,
                      height: 28,
                      alignment: Alignment.center,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: cs.primary.withOpacity(0.12),
                      ),
                      child: Text(
                        '${i + 1}',
                        style: TextStyle(
                            fontSize: 11,
                            fontWeight: FontWeight.bold,
                            color: cs.primary),
                      ),
                    ),
                    title: Text(e.displayTicker,
                        style: const TextStyle(
                            fontWeight: FontWeight.w600, fontSize: 13)),
                    subtitle: Consumer(
                      builder: (_, tileRef, __) {
                        final funds = tileRef
                                .watch(entityFundsProvider(e.id))
                                .valueOrNull ??
                            const [];
                        return Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Padding(
                              padding: EdgeInsets.only(
                                  top: 4, bottom: funds.isNotEmpty ? 2 : 0),
                              child: ClipRRect(
                                borderRadius: BorderRadius.circular(2),
                                child: LinearProgressIndicator(
                                  value: ratio,
                                  minHeight: 3,
                                  backgroundColor:
                                      Colors.grey.withOpacity(0.12),
                                  valueColor:
                                      AlwaysStoppedAnimation<Color>(cs.primary),
                                ),
                              ),
                            ),
                            if (funds.isNotEmpty)
                              Text(
                                _formatFundCodes(funds),
                                style: TextStyle(
                                  fontSize: 10,
                                  color: cs.onSurface.withOpacity(0.45),
                                ),
                                overflow: TextOverflow.ellipsis,
                              ),
                          ],
                        );
                      },
                    ),
                    trailing: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          '${e.fundCount}',
                          style: TextStyle(
                            fontWeight: FontWeight.bold,
                            fontSize: 15,
                            color: cs.primary,
                          ),
                        ),
                        const SizedBox(width: 3),
                        Text(
                          kLabelFundsUnit,
                          style: TextStyle(
                              fontSize: 10,
                              color: cs.onSurface.withOpacity(0.5)),
                        ),
                        const SizedBox(width: 4),
                        GestureDetector(
                          onTap: () =>
                              ref.read(watchlistProvider.notifier).toggle(e.id),
                          child: Icon(
                            inWatchlist
                                ? Icons.star_rounded
                                : Icons.star_outline_rounded,
                            size: 20,
                            color: inWatchlist
                                ? Colors.amber.shade600
                                : Colors.grey.shade400,
                          ),
                        ),
                      ],
                    ),
                    onTap: () => Navigator.of(ctx).push(
                      MaterialPageRoute<void>(
                        builder: (_) => StockDetailScreen(entity: e),
                      ),
                    ),
                  );
                },
              ),
            );
          },
          loading: () => const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(child: CircularProgressIndicator()),
          ),
          error: (e, _) => Padding(
            padding: const EdgeInsets.all(12),
            child: Text('Yüklenemedi: $e',
                style: TextStyle(
                    fontSize: 12, color: Theme.of(context).colorScheme.error)),
          ),
        ),
      ],
    );
  }
}

String _formatFundCodes(List<FundHolding> funds) {
  final codes = funds.take(3).map((f) => f.fundCode).join(' · ');
  return funds.length > 3 ? '$codes +${funds.length - 3}' : codes;
}
