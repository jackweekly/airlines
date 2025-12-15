import 'package:flutter/material.dart';

import '../models/owned_craft.dart';
import '../models/route_info.dart';

class KpiRow extends StatelessWidget {
  const KpiRow({
    super.key,
    required this.cash,
    required this.tick,
    required this.routes,
    required this.fleet,
  });

  final double cash;
  final int tick;
  final List<RouteInfo> routes;
  final List<OwnedCraft> fleet;

  @override
  Widget build(BuildContext context) {
    final leaseCost = fleet.fold<double>(
      0,
      (sum, f) =>
          f.ownershipType.toLowerCase() == 'leased' ? sum + f.monthlyCost : sum,
    );
    final routeProfit = routes.fold<double>(
      0,
      (sum, r) => sum + r.profit.toDouble(),
    );
    final losingCashFlow = routeProfit - leaseCost < 0;

    final cards = [
      _kpiChip('Cash', '\$${cash.toStringAsFixed(0)}', danger: losingCashFlow),
      _kpiChip('Tick', '$tick'),
      if (routes.isNotEmpty)
        _kpiChip(
          'Avg load',
          '${(routes.map((e) => e.load).fold<double>(0, (a, b) => a + b) / routes.length * 100).toStringAsFixed(0)}%',
        ),
    ];

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: cards
            .map(
              (c) =>
                  Padding(padding: const EdgeInsets.only(right: 6), child: c),
            )
            .toList(),
      ),
    );
  }

  Widget _kpiChip(String label, String value, {bool danger = false}) {
    final valueColor = danger ? Colors.redAccent : Colors.white;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white.withOpacity(0.08),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.white24),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label.toUpperCase(),
            style: const TextStyle(
              color: Colors.white54,
              fontSize: 10,
              letterSpacing: 0.5,
            ),
          ),
          Text(
            value,
            style: TextStyle(
              color: valueColor,
              fontWeight: FontWeight.w600,
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }
}
