import 'package:flutter/material.dart';

import '../models/aircraft_template.dart';
import '../models/owned_craft.dart';
import '../models/route_info.dart';

class FloatingPanel extends StatelessWidget {
  const FloatingPanel({
    super.key,
    required this.isRoutes,
    required this.showAnalysis,
    required this.onClose,
    required this.onBackFromAnalysis,
    required this.routeForm,
    required this.ctaRow,
    required this.errorMessage,
    required this.analysisTable,
    required this.routes,
    required this.fleet,
    required this.templates,
    required this.onOpenCatalog,
    required this.routeTileBuilder,
    required this.fleetTileBuilder,
  });

  final bool isRoutes;
  final bool showAnalysis;
  final VoidCallback onClose;
  final VoidCallback onBackFromAnalysis;
  final Widget routeForm;
  final Widget ctaRow;
  final Widget? errorMessage;
  final Widget analysisTable;
  final List<RouteInfo> routes;
  final List<OwnedCraft> fleet;
  final List<AircraftTemplate> templates;
  final VoidCallback onOpenCatalog;
  final Widget Function(RouteInfo) routeTileBuilder;
  final Widget Function(OwnedCraft) fleetTileBuilder;

  @override
  Widget build(BuildContext context) {
    final height = MediaQuery.of(context).size.height * 0.6;
    const width = 450.0;
    return Align(
      alignment: Alignment.bottomLeft,
      child: Material(
        color: Colors.black.withOpacity(0.85),
        borderRadius: BorderRadius.circular(14),
        child: Container(
          height: height,
          width: width,
          padding: const EdgeInsets.all(10),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Row(
                    children: [
                      if (showAnalysis)
                        IconButton(
                          icon: const Icon(
                            Icons.arrow_back,
                            color: Colors.white,
                          ),
                          onPressed: onBackFromAnalysis,
                          padding: EdgeInsets.zero,
                          constraints: const BoxConstraints(),
                        ),
                      if (showAnalysis) const SizedBox(width: 8),
                      Text(
                        showAnalysis
                            ? 'Strategy Analyzer'
                            : (isRoutes ? 'Routes' : 'My Fleet'),
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.bold,
                          fontSize: 20,
                        ),
                      ),
                    ],
                  ),
                  IconButton(
                    icon: const Icon(Icons.close, color: Colors.white70),
                    onPressed: onClose,
                  ),
                ],
              ),
              const SizedBox(height: 6),
              if (isRoutes)
                Expanded(
                  child: showAnalysis
                      ? analysisTable
                      : Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            routeForm,
                            const SizedBox(height: 8),
                            ctaRow,
                            if (errorMessage != null) errorMessage!,
                            const SizedBox(height: 12),
                            _sectionTitle('Active routes'),
                            const SizedBox(height: 4),
                            Expanded(
                              child: routes.isEmpty
                                  ? const Center(
                                      child: Text(
                                        'No routes yet',
                                        style: TextStyle(color: Colors.white54),
                                      ),
                                    )
                                  : ListView.builder(
                                      itemCount: routes.length,
                                      itemBuilder: (context, i) =>
                                          routeTileBuilder(routes[i]),
                                    ),
                            ),
                          ],
                        ),
                )
              else
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const SizedBox(height: 4),
                      SizedBox(
                        width: double.infinity,
                        child: ElevatedButton.icon(
                          onPressed: templates.isEmpty ? null : onOpenCatalog,
                          style: ElevatedButton.styleFrom(
                            backgroundColor: Colors.tealAccent,
                            foregroundColor: Colors.black,
                            padding: const EdgeInsets.symmetric(vertical: 12),
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(10),
                            ),
                          ),
                          icon: const Icon(Icons.add),
                          label: const Text('Purchase aircraft'),
                        ),
                      ),
                      const SizedBox(height: 12),
                      Expanded(
                        child: fleet.isEmpty
                            ? const Center(
                                child: Text(
                                  'No aircraft',
                                  style: TextStyle(color: Colors.white54),
                                ),
                              )
                            : ListView.builder(
                                itemCount: fleet.length,
                                itemBuilder: (context, i) =>
                                    fleetTileBuilder(fleet[i]),
                              ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _sectionTitle(String title) {
    return Text(
      title,
      style: const TextStyle(
        color: Colors.white70,
        fontWeight: FontWeight.w700,
        fontSize: 13,
      ),
    );
  }
}
