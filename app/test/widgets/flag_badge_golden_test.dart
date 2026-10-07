import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:neyialiyorlar/core/model/metric.dart';
import 'package:neyialiyorlar/widgets/flag_badge.dart';

void main() {
  group('FlagBadge Golden Tests', () {
    testWidgets('FlagBadge fresh renders correctly', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(200, 100);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Center(
              child: FlagBadge(flag: Confidence.fresh),
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(FlagBadge),
        matchesGoldenFile('goldens/flag_badge_fresh.png'),
      );
    });

    testWidgets('FlagBadge stale renders correctly', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(200, 100);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Center(
              child: FlagBadge(flag: Confidence.stale),
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(FlagBadge),
        matchesGoldenFile('goldens/flag_badge_stale.png'),
      );
    });

    testWidgets('FlagBadge approx renders correctly', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(200, 100);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Center(
              child: FlagBadge(flag: Confidence.approx),
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(FlagBadge),
        matchesGoldenFile('goldens/flag_badge_approx.png'),
      );
    });

    testWidgets('FlagBadge small size renders correctly', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(200, 100);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Center(
              child: FlagBadge(
                flag: Confidence.fresh,
                size: 16,
              ),
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(FlagBadge),
        matchesGoldenFile('goldens/flag_badge_small.png'),
      );
    });
  });

  group('Gap Rendering Golden Tests', () {
    testWidgets('Gap value renders as dash', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(120, 60);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: DenseCellTest(
              label: 'NIMVI',
              value: null, // Gap
              flag: Confidence.stale,
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(DenseCellTest),
        matchesGoldenFile('goldens/dense_cell_gap.png'),
      );
    });

    testWidgets('Gap value is never interpolated', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(120, 60);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: DenseCellTest(
              label: 'Test',
              value: null,
              flag: Confidence.stale,
            ),
          ),
        ),
      );

      // Verify dash is rendered, not zero
      expect(find.text('—'), findsOneWidget);
      expect(find.text('0'), findsNothing);
    });

    testWidgets('Present value renders correctly', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(120, 60);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: DenseCellTest(
              label: 'NIMVI',
              value: 1.83,
              flag: Confidence.fresh,
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(DenseCellTest),
        matchesGoldenFile('goldens/dense_cell_value.png'),
      );
    });

    testWidgets('Gap renders with distinct visual treatment', (tester) async {
      tester.binding.window.physicalSizeTestValue = const Size(120, 60);
      addTearDown(tester.binding.window.clearPhysicalSizeTestValue);

      // Render gap and present side-by-side to verify distinct appearance
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: Row(
              children: [
                DenseCellTest(
                  label: 'Gap',
                  value: null,
                  flag: Confidence.stale,
                ),
                SizedBox(width: 8),
                DenseCellTest(
                  label: 'Value',
                  value: 1.83,
                  flag: Confidence.fresh,
                ),
              ],
            ),
          ),
        ),
      );

      await expectLater(
        find.byType(Row),
        matchesGoldenFile('goldens/dense_cells_gap_vs_value.png'),
      );
    });
  });
}

/// Test widget for golden tests - renders DenseCell equivalent
class DenseCellTest extends StatelessWidget {
  const DenseCellTest({super.key,
    required this.label,
    required this.value,
    required this.flag,
  });
  final String label;
  final double? value;
  final Confidence flag;

  @override
  Widget build(BuildContext context) {
    final valueText = value == null ? '—' : value!.toStringAsFixed(2);

    return Container(
      width: 120,
      height: 60,
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        border: Border.all(color: Colors.grey),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                label,
                style: TextStyle(fontSize: 10, color: Colors.grey.shade600),
                overflow: TextOverflow.ellipsis,
              ),
            ],
          ),
          const SizedBox(height: 2),
          Text(
            valueText,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.bold,
              color: value == null ? Colors.grey : Colors.black87,
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}
