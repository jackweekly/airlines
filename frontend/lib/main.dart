import 'dart:html' as html;
import 'dart:ui_web' as ui_web;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_map_marker_cluster/flutter_map_marker_cluster.dart';
import 'package:latlong2/latlong.dart';

const _mapboxToken =
    'pk.eyJ1IjoiamFja3dlZWtseSIsImEiOiJjbWc0amtmaG8wd2NpMmpwdXF0a2E4c3JjIn0.BXoED0qe_UmZVeWKWHe_6Q';
const _mapboxStyles = {
  'Satellite': 'mapbox/satellite-v9',
  'Light': 'mapbox/light-v11',
  'Streets': 'mapbox/streets-v12',
  'Dark': 'mapbox/dark-v11',
};

void main() {
  runApp(const AirlineBuilderApp());
}

class AirlineBuilderApp extends StatelessWidget {
  const AirlineBuilderApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Airline Builder',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.teal),
        useMaterial3: true,
      ),
      home: kIsWeb ? const MapboxGlobeWeb() : const GlobeScreen(),
    );
  }
}

class GlobeScreen extends StatefulWidget {
  const GlobeScreen({super.key});

  @override
  State<GlobeScreen> createState() => _GlobeScreenState();
}

class _GlobeScreenState extends State<GlobeScreen> {
  final MapController _mapController = MapController();
  String _currentStyle = _mapboxStyles.keys.first;
  double _currentZoom = 2.5;

  @override
  Widget build(BuildContext context) {
    const iad = LatLng(38.9531, -77.4565); // Dulles
    const dca = LatLng(38.8512, -77.0402); // Reagan

    return Scaffold(
      body: ColoredBox(
        color: Colors.black,
        child: SafeArea(
          child: FlutterMap(
            options: MapOptions(
              initialCenter: const LatLng(0, 0),
              initialZoom: 2.5,
              minZoom: 1,
              maxZoom: 18,
              onMapReady: () {
                // Center roughly on US east coast to show markers
                _mapController.move(const LatLng(38.9, -77.2), 8);
              },
              onMapEvent: (evt) {
                if (evt is MapEventMoveEnd) {
                  setState(() {
                    _currentZoom = evt.camera.zoom;
                  });
                }
              },
            ),
            mapController: _mapController,
            children: [
              // Primary: Mapbox raster tiles
              TileLayer(
                key: ValueKey(_currentStyle),
                urlTemplate:
                    'https://api.mapbox.com/styles/v1/{id}/tiles/256/{z}/{x}/{y}@2x?access_token={accessToken}',
                additionalOptions: {
                  'id': _mapboxStyles[_currentStyle]!,
                  'accessToken': _mapboxToken,
                },
                // Use 512px tiles with zoomOffset to reduce over-sharp labels when zoomed out
                tileSize: 512,
                zoomOffset: -1,
                retinaMode: false,
                maxNativeZoom: 17,
                maxZoom: 19,
                userAgentPackageName: 'airline_builder_frontend',
              ),
              // Fallback debug layer (OSM) to verify rendering if Mapbox is blocked
              // Uncomment and set visible=true to test fallback tiles if Mapbox fails.
              // TileLayer(
              //   urlTemplate: 'https://tile.openstreetmap.org/{z}/{x}/{y}.png',
              //   retinaMode: true,
              //   maxZoom: 19,
              //   userAgentPackageName: 'airline_builder_frontend',
              // ),
              MarkerClusterLayerWidget(
                options: MarkerClusterLayerOptions(
                  markers: _airportMarkers(iad, dca),
                  maxClusterRadius: 45,
                  disableClusteringAtZoom: 8,
                  size: const Size(36, 36),
                  builder: (context, markers) => Container(
                    decoration: const BoxDecoration(
                      color: Colors.teal,
                      shape: BoxShape.circle,
                    ),
                    child: Center(
                      child: Text(
                        '${markers.length}',
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () {},
        label: DropdownButtonHideUnderline(
          child: DropdownButton<String>(
            value: _currentStyle,
            items: _mapboxStyles.keys
                .map(
                  (name) => DropdownMenuItem(
                    value: name,
                    child: Text(name),
                  ),
                )
                .toList(),
            onChanged: (val) {
              if (val == null) return;
              final center = _mapController.camera.center;
              final zoom = _mapController.camera.zoom;
              setState(() {
                _currentStyle = val;
              });
              // Recenter to force tile reload on style change
              _mapController.move(center, zoom);
            },
          ),
        ),
        icon: const Icon(Icons.map),
      ),
    );
  }

  List<Marker> _airportMarkers(LatLng iad, LatLng dca) {
    // TODO: replace with real airport list; for now gate by zoom as an example
    final markers = <Marker>[
      Marker(
        point: iad,
        width: 18,
        height: 18,
        child: const Icon(Icons.location_on, color: Colors.red, size: 18),
      ),
      Marker(
        point: dca,
        width: 18,
        height: 18,
        child: const Icon(Icons.location_on, color: Colors.blue, size: 18),
      ),
    ];

    // Zoom-based filtering could go here (e.g., show only hubs when zoom < 5).
    return markers;
  }
}

class MapboxGlobeWeb extends StatefulWidget {
  const MapboxGlobeWeb({super.key});

  @override
  State<MapboxGlobeWeb> createState() => _MapboxGlobeWebState();
}

class _MapboxGlobeWebState extends State<MapboxGlobeWeb> {
  late final String _viewId;
  bool _mapReady = false;

  @override
  void initState() {
    super.initState();
    _viewId = 'mapbox-globe-${DateTime.now().millisecondsSinceEpoch}';
    // ignore: undefined_prefixed_name
    ui_web.platformViewRegistry.registerViewFactory(_viewId, (int _) {
      final iframe = html.IFrameElement()
        ..style.border = 'none'
        ..style.width = '100%'
        ..style.height = '100%'
        ..src =
            'map.html?token=${Uri.encodeComponent(_mapboxToken)}&style=mapbox://styles/mapbox/dark-v11&center=-77.2,38.9&zoom=2.5';
      iframe.onLoad.listen((_) {
        if (mounted) {
          setState(() {
            _mapReady = true;
          });
        }
      });
      return iframe;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        Positioned.fill(child: HtmlElementView(viewType: _viewId)),
        if (!_mapReady)
          const Positioned(
            left: 12,
            bottom: 12,
            child: ColoredBox(
              color: Colors.black54,
              child: Padding(
                padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                child: Text(
                  'Loading globe…',
                  style: TextStyle(color: Colors.white),
                ),
              ),
            ),
          ),
      ],
    );
  }
}
