import 'package:flutter/material.dart';

import '../models/route_analysis_result.dart';

class AnalysisTable extends StatelessWidget {
  const AnalysisTable({
    super.key,
    required this.results,
    required this.onSelect,
  });

  final List<RouteAnalysisResult> results;
  final void Function(RouteAnalysisResult) onSelect;

  @override
  Widget build(BuildContext context) {
    final sorted = [...results]
      ..sort((a, b) => b.dailyProfit.compareTo(a.dailyProfit));

    if (sorted.isEmpty) {
      return const Center(
        child: Text(
          'No viable aircraft found',
          style: TextStyle(color: Colors.white54),
        ),
      );
    }

    return SingleChildScrollView(
      scrollDirection: Axis.vertical,
      child: DataTable(
        headingRowColor: WidgetStateProperty.all(Colors.white10),
        dataRowColor: WidgetStateProperty.all(Colors.transparent),
        columnSpacing: 12,
        columns: const [
          DataColumn(
            label: Text(
              'Aircraft',
              style: TextStyle(color: Colors.white70, fontSize: 12),
            ),
          ),
          DataColumn(
            label: Text(
              'Freq',
              style: TextStyle(color: Colors.white70, fontSize: 12),
            ),
          ),
          DataColumn(
            label: Text(
              'Profit',
              style: TextStyle(color: Colors.white70, fontSize: 12),
            ),
          ),
          DataColumn(
            label: Text(
              'ROI',
              style: TextStyle(color: Colors.white70, fontSize: 12),
            ),
          ),
          DataColumn(label: Text('')),
        ],
        rows: sorted
            .map(
              (r) => DataRow(
                cells: [
                  DataCell(
                    Text(
                      r.aircraftType,
                      style: const TextStyle(color: Colors.white),
                    ),
                  ),
                  DataCell(
                    Text(
                      '${r.frequency.toInt()}x',
                      style: const TextStyle(color: Colors.white70),
                    ),
                  ),
                  DataCell(
                    Text(
                      '\$${r.dailyProfit.toStringAsFixed(0)}',
                      style: TextStyle(
                        color: r.dailyProfit >= 0
                            ? Colors.tealAccent
                            : Colors.redAccent,
                        fontWeight: r == sorted.first
                            ? FontWeight.w700
                            : FontWeight.w500,
                      ),
                    ),
                  ),
                  DataCell(
                    Text(
                      '${r.roiScore.toStringAsFixed(0)}%',
                      style: const TextStyle(color: Colors.white70),
                    ),
                  ),
                  DataCell(
                    TextButton(
                      onPressed: () => onSelect(r),
                      child: const Text('Select'),
                    ),
                  ),
                ],
              ),
            )
            .toList(),
      ),
    );
  }
}
