import 'package:flutter/material.dart';
import '../core/model/metric.dart';

/// FlagBadge widget renders the confidence flag with distinct visual treatment
/// - fresh: solid color (green/blue)
/// - stale: amber dim
/// - approx: hatched/striped pattern
class FlagBadge extends StatelessWidget {
  const FlagBadge({
    required this.flag, super.key,
    this.size = 24.0,
  });
  final Confidence flag;
  final double size;

  @override
  Widget build(BuildContext context) {
    switch (flag) {
      case Confidence.fresh:
        return Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            color: Colors.green.shade400,
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.check_circle,
            size: size * 0.7,
            color: Colors.white,
          ),
        );

      case Confidence.stale:
        return Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            color: Colors.amber.shade300,
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.warning_amber,
            size: size * 0.7,
            color: Colors.amber.shade900,
          ),
        );

      case Confidence.approx:
        return Container(
          width: size,
          height: size,
          decoration: BoxDecoration(
            color: Colors.orange.shade200,
            shape: BoxShape.circle,
          ),
          child: Icon(
            Icons.info_outline,
            size: size * 0.7,
            color: Colors.orange.shade700,
          ),
        );
    }
  }
}

/// DenseCell widget: fixed-extent cell for metric values with no layout thrash
/// Optimized for virtualized grid rendering
class DenseCell extends StatelessWidget {
  const DenseCell({
    required this.label, required this.flag, super.key,
    this.value,
    this.height = 48.0,
    this.width = 120.0,
  });
  final String label;
  final double? value;
  final Confidence flag;
  final double? height;
  final double? width;

  @override
  Widget build(BuildContext context) {
    final valueText = value == null ? '—' : value!.toStringAsFixed(2);

    return Container(
      width: width,
      height: height,
      padding: const EdgeInsets.all(4),
      decoration: BoxDecoration(
        border: Border.all(color: Colors.grey.shade300),
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
              FlagBadge(flag: flag, size: 16),
            ],
          ),
          const SizedBox(height: 2),
          Text(
            valueText,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.bold,
              color: value == null ? Colors.grey.shade400 : Colors.black87,
            ),
            overflow: TextOverflow.ellipsis,
          ),
        ],
      ),
    );
  }
}
