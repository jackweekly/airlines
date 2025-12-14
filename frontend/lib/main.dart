import 'dart:convert';
import 'dart:html' as html;
import 'dart:ui_web' as ui_web;

import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_map/flutter_map.dart';
import 'package:flutter_map_marker_cluster/flutter_map_marker_cluster.dart';
import 'package:http/http.dart' as http;
import 'package:latlong2/latlong.dart';
import 'package:pointer_interceptor/pointer_interceptor.dart';

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
      home: kIsWeb
          ? MapboxGlobeWeb(styleId: _mapboxStyles.values.first)
          : const GlobeScreen(),
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
  List<Airport> _airports = const [];
  bool _loadingAirports = false;
  String? _airportsError;
  bool _showSettings = false;

  @override
  void initState() {
    super.initState();
    _fetchAirports();
  }

  @override
  Widget build(BuildContext context) {
    const iad = LatLng(38.9531, -77.4565); // Dulles
    const dca = LatLng(38.8512, -77.0402); // Reagan

    return Scaffold(
      body: Shortcuts(
        shortcuts: <LogicalKeySet, Intent>{
          LogicalKeySet(LogicalKeyboardKey.escape): const ActivateIntent(),
        },
        child: Actions(
          actions: <Type, Action<Intent>>{
            ActivateIntent: CallbackAction<Intent>(
              onInvoke: (intent) {
                setState(() {
                  _showSettings = !_showSettings;
                });
                return null;
              },
            ),
          },
          child: Focus(
            autofocus: true,
            child: Stack(
              children: [
                ColoredBox(
                  color: Colors.black,
                  child: SafeArea(
                    child: FlutterMap(
                      options: MapOptions(
                        initialCenter: const LatLng(0, 0),
                        initialZoom: 2.5,
                        minZoom: 1,
                        maxZoom: 18,
                        onMapReady: () {
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
                        TileLayer(
                          key: ValueKey(_currentStyle),
                          urlTemplate:
                              'https://api.mapbox.com/styles/v1/{id}/tiles/256/{z}/{x}/{y}@2x?access_token={accessToken}',
                          additionalOptions: {
                            'id': _mapboxStyles[_currentStyle]!,
                            'accessToken': _mapboxToken,
                          },
                          tileSize: 512,
                          zoomOffset: -1,
                          retinaMode: false,
                          maxNativeZoom: 17,
                          maxZoom: 19,
                          userAgentPackageName: 'airline_builder_frontend',
                        ),
                        MarkerClusterLayerWidget(
                          options: MarkerClusterLayerOptions(
                            markers: _airportMarkers(),
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
                if (_loadingAirports)
                  const Positioned(
                    left: 12,
                    top: 12,
                    child: ColoredBox(
                      color: Colors.black54,
                      child: Padding(
                        padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                        child: Text(
                          'Loading airports…',
                          style: TextStyle(color: Colors.white),
                        ),
                      ),
                    ),
                  ),
                if (_airportsError != null)
                  Positioned(
                    left: 12,
                    top: 12,
                    child: ColoredBox(
                      color: Colors.redAccent.withOpacity(0.9),
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                        child: Text(
                          _airportsError!,
                          style: const TextStyle(color: Colors.white),
                        ),
                      ),
                    ),
                  ),
                // Compass / reset button
                Positioned(
                  right: 16,
                  top: 16,
                  child: Column(
                    children: [
                      FloatingActionButton(
                        heroTag: 'compass',
                        mini: true,
                        onPressed: _resetView,
                        child: const Icon(Icons.explore),
                      ),
                      const SizedBox(height: 8),
                      FloatingActionButton(
                        heroTag: 'settings',
                        mini: true,
                        onPressed: () {
                          setState(() {
                            _showSettings = true;
                          });
                        },
                        child: const Icon(Icons.settings),
                      ),
                    ],
                  ),
                ),
                if (_showSettings) _buildSettingsPanel(),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildSettingsPanel() {
    return Positioned(
      right: 16,
      bottom: 16,
      child: Material(
        color: Colors.black87,
        borderRadius: BorderRadius.circular(12),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 280),
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: [
                    const Text('Settings', style: TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
                    IconButton(
                      onPressed: () => setState(() => _showSettings = false),
                      icon: const Icon(Icons.close, color: Colors.white),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                const Text('Theme / Style', style: TextStyle(color: Colors.white70)),
                DropdownButtonHideUnderline(
                  child: DropdownButton<String>(
                    dropdownColor: Colors.black87,
                    value: _currentStyle,
                    items: _mapboxStyles.keys
                        .map(
                          (name) => DropdownMenuItem(
                            value: name,
                            child: Text(name, style: const TextStyle(color: Colors.white)),
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
                      _mapController.move(center, zoom);
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _resetView() {
    _mapController.rotate(0);
    _mapController.move(const LatLng(38.9, -77.2), 8);
  }

  Future<void> _fetchAirports() async {
    setState(() {
      _loadingAirports = true;
      _airportsError = null;
    });
    try {
      final resp = await http.get(Uri.parse('http://localhost:4000/airports'));
      if (resp.statusCode == 200) {
        final List<dynamic> data = json.decode(resp.body);
        final parsed = data
            .map((e) => Airport.fromJson(e as Map<String, dynamic>))
            .toList();
        setState(() {
          _airports = parsed;
        });
      } else {
        setState(() {
          _airportsError = 'Failed to load airports (${resp.statusCode})';
        });
      }
    } catch (e) {
      setState(() {
        _airportsError = 'Failed to load airports: $e';
      });
    } finally {
      if (mounted) {
        setState(() {
          _loadingAirports = false;
        });
      }
    }
  }

  List<Marker> _airportMarkers() {
    if (_airports.isEmpty) return const [];

    Iterable<Airport> filtered = _airports;
    if (_currentZoom < 5) {
      filtered = filtered.where(
          (a) => a.type == 'large_airport' || a.type == 'medium_airport');
    } else if (_currentZoom < 7) {
      filtered = filtered.where((a) => a.type != 'small_airport');
    }

    return filtered
        .map(
          (a) => Marker(
            point: LatLng(a.lat, a.lon),
            width: 10,
            height: 10,
            child: const Icon(Icons.circle, color: Colors.white, size: 10),
          ),
        )
        .toList();
  }
}

class MapboxGlobeWeb extends StatefulWidget {
  const MapboxGlobeWeb({super.key, this.styleId = 'mapbox/dark-v11'});

  final String styleId;

  @override
  State<MapboxGlobeWeb> createState() => _MapboxGlobeWebState();
}

class _MapboxGlobeWebState extends State<MapboxGlobeWeb> {
  late final String _viewId;
  bool _mapReady = false;
  bool _showSettings = false;
  late String _currentStyle;
  late final html.IFrameElement _iframe;
  double _cash = 0;
  int _tick = 0;
  List<RouteInfo> _routes = const [];
  List<OwnedCraft> _fleet = const [];
  final TextEditingController _fromCtrl = TextEditingController();
  final TextEditingController _toCtrl = TextEditingController();
  String _aircraftId = 'A320';
  int _freqPerDay = 1;
  bool _busy = false;
  String? _error;
  final Map<String, double> _prices = const {'A320': 98_000_000, 'B738': 96_000_000, 'E190': 52_000_000};
  final Map<String, int> _lead = const {'A320': 8, 'B738': 8, 'E190': 6};
  bool _oneWay = false;

  @override
  void initState() {
    super.initState();
    _currentStyle = widget.styleId;
    _viewId = 'mapbox-globe-${DateTime.now().millisecondsSinceEpoch}';
    _iframe = html.IFrameElement()
      ..style.border = 'none'
      ..style.width = '100%'
      ..style.height = '100%'
      ..src = _srcForStyle(_currentStyle);
    _iframe.onLoad.listen((_) {
      if (mounted) {
        setState(() {
          _mapReady = true;
        });
        _loadState();
      }
    });
    // ignore: undefined_prefixed_name
    ui_web.platformViewRegistry.registerViewFactory(_viewId, (int _) => _iframe);
  }

  @override
  Widget build(BuildContext context) {
    return Shortcuts(
      shortcuts: <LogicalKeySet, Intent>{
        LogicalKeySet(LogicalKeyboardKey.escape): const ActivateIntent(),
      },
      child: Actions(
        actions: <Type, Action<Intent>>{
          ActivateIntent: CallbackAction<Intent>(
            onInvoke: (intent) {
              setState(() => _showSettings = !_showSettings);
              return null;
            },
          ),
        },
        child: Focus(
          autofocus: true,
          child: Stack(
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
              Positioned(
                right: 16,
                top: 16,
                child: PointerInterceptor(
                  child: Column(
                    children: [
                      FloatingActionButton(
                        heroTag: 'globe-reset',
                        mini: true,
                        onPressed: _resetGlobe,
                        child: const Icon(Icons.explore),
                      ),
                      const SizedBox(height: 8),
                      FloatingActionButton(
                        heroTag: 'globe-settings',
                        mini: true,
                        onPressed: () => setState(() => _showSettings = true),
                        child: const Icon(Icons.settings),
                      ),
                    ],
                  ),
                ),
              ),
              if (_showSettings) _buildSettingsPanel(),
              Positioned(
                left: 16,
                bottom: 16,
                child: PointerInterceptor(
                  child: _buildRoutePanel(),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildSettingsPanel() {
    return Positioned(
      right: 16,
      top: 64,
      child: PointerInterceptor(
        child: Material(
          color: Colors.black87,
          borderRadius: BorderRadius.circular(12),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 280),
            child: Padding(
              padding: const EdgeInsets.all(12),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      const Text('Settings', style: TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600)),
                      IconButton(
                        onPressed: () => setState(() => _showSettings = false),
                        icon: const Icon(Icons.close, color: Colors.white),
                      ),
                    ],
                  ),
                const SizedBox(height: 8),
                const Text('Theme / Style', style: TextStyle(color: Colors.white70)),
                const SizedBox(height: 8),
                ..._mapboxStyles.entries.map((entry) {
                  final isSelected = entry.value == _currentStyle;
                  return ListTile(
                    contentPadding: EdgeInsets.zero,
                    dense: true,
                    leading: Icon(
                      isSelected ? Icons.radio_button_checked : Icons.radio_button_unchecked,
                      color: Colors.white,
                    ),
                    title: Text(entry.key, style: const TextStyle(color: Colors.white)),
                    onTap: () {
                      setState(() {
                        _currentStyle = entry.value;
                        _mapReady = false;
                      });
                      Future.microtask(() {
                        _iframe.src = _srcForStyle(entry.value);
                      });
                    },
                  );
                }),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  String _styleNameFor(String styleId) {
    return _mapboxStyles.entries
        .firstWhere((e) => e.value == styleId, orElse: () => _mapboxStyles.entries.first)
        .key;
  }

  String _srcForStyle(String styleId) {
    return 'map.html?token=${Uri.encodeComponent(_mapboxToken)}&style=$styleId&center=-77.2,38.9&zoom=2.5';
  }

  void _resetGlobe() {
    setState(() {
      _mapReady = false;
    });
    _iframe.src = _srcForStyle(_currentStyle);
  }

  Widget _buildRoutePanel() {
    return Material(
      elevation: 4,
      color: Colors.black.withOpacity(0.85),
      borderRadius: BorderRadius.circular(12),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 320),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  const Text('Routes', style: TextStyle(color: Colors.white, fontWeight: FontWeight.w700)),
                  IconButton(
                    icon: const Icon(Icons.refresh, color: Colors.white70),
                    onPressed: _busy ? null : _loadState,
                  )
                ],
              ),
              Text('Cash: \$${_cash.toStringAsFixed(0)}', style: const TextStyle(color: Colors.white70)),
              Text('Tick: $_tick', style: const TextStyle(color: Colors.white70)),
              const SizedBox(height: 8),
              Row(
                children: [
                  Flexible(
                    child: TextField(
                      controller: _fromCtrl,
                      decoration: const InputDecoration(
                        labelText: 'From (IATA/ICAO)',
                        labelStyle: TextStyle(color: Colors.white70),
                        hintText: 'e.g. IAD',
                        hintStyle: TextStyle(color: Colors.white38),
                      ),
                      style: const TextStyle(color: Colors.white),
                    ),
                  ),
                  const SizedBox(width: 8),
                  Flexible(
                    child: TextField(
                      controller: _toCtrl,
                      decoration: const InputDecoration(
                        labelText: 'To',
                        labelStyle: TextStyle(color: Colors.white70),
                        hintText: 'e.g. LAX',
                        hintStyle: TextStyle(color: Colors.white38),
                      ),
                      style: const TextStyle(color: Colors.white),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: 8),
              DropdownButtonFormField<String>(
                value: _aircraftId,
                dropdownColor: Colors.black87,
                decoration: const InputDecoration(
                  labelText: 'Aircraft',
                  labelStyle: TextStyle(color: Colors.white70),
                ),
                items: const [
                  DropdownMenuItem(value: 'A320', child: Text('A320', style: TextStyle(color: Colors.white))),
                  DropdownMenuItem(value: 'B738', child: Text('B737-800', style: TextStyle(color: Colors.white))),
                  DropdownMenuItem(value: 'E190', child: Text('E190', style: TextStyle(color: Colors.white))),
                ],
                onChanged: (v) => setState(() => _aircraftId = v ?? 'A320'),
              ),
              const SizedBox(height: 8),
              Row(
                children: [
                  const Text('Freq/day', style: TextStyle(color: Colors.white70)),
                  Expanded(
                    child: Slider(
                      value: _freqPerDay.toDouble(),
                      min: 1,
                      max: 10,
                      divisions: 9,
                      label: '$_freqPerDay',
                      onChanged: (v) => setState(() => _freqPerDay = v.toInt()),
                    ),
                  ),
                ],
              ),
              Row(
                children: [
                  Checkbox(
                    value: _oneWay,
                    onChanged: (v) => setState(() => _oneWay = v ?? false),
                    fillColor: MaterialStateProperty.all(Colors.white24),
                    checkColor: Colors.black,
                  ),
                  const Text('One-way only', style: TextStyle(color: Colors.white70)),
                ],
              ),
              const SizedBox(height: 8),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  onPressed: _busy ? null : _createRoute,
                  child: _busy ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2)) : const Text('Create route'),
                ),
              ),
              const SizedBox(height: 4),
              SizedBox(
                width: double.infinity,
                child: OutlinedButton(
                  onPressed: _busy ? null : _tickOnce,
                  child: const Text('Advance tick'),
                ),
              ),
              if (_error != null)
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(_error!, style: const TextStyle(color: Colors.redAccent, fontSize: 12)),
                ),
              const SizedBox(height: 8),
              const Text('Active routes', style: TextStyle(color: Colors.white70)),
              SizedBox(
                height: 140,
                child: _routes.isEmpty
                    ? const Center(child: Text('No routes yet', style: TextStyle(color: Colors.white54)))
                    : ListView.builder(
                        itemCount: _routes.length,
                        itemBuilder: (context, i) {
                          final r = _routes[i];
                          return ListTile(
                            dense: true,
                            title: Text('${r.from} → ${r.to} (${r.aircraftId})', style: const TextStyle(color: Colors.white)),
                            subtitle: Text(
                              'Freq ${r.freq}/day | Block ${r.blockMins.toStringAsFixed(0)}m | Load ${(r.load * 100).toStringAsFixed(0)}% | Fees \$${r.landingFees.toStringAsFixed(0)}/leg | Rev \$${r.rev.toStringAsFixed(0)} Cost \$${r.cost.toStringAsFixed(0)} Profit \$${r.profit.toStringAsFixed(0)}${r.curfewBlocked ? ' | Curfew blocked' : ''}',
                              style: const TextStyle(color: Colors.white70, fontSize: 12),
                            ),
                          );
                        },
                      ),
              ),
              const SizedBox(height: 8),
              const Text('Fleet', style: TextStyle(color: Colors.white70)),
              SizedBox(
                height: 100,
                child: _fleet.isEmpty
                    ? const Center(child: Text('No aircraft', style: TextStyle(color: Colors.white54)))
                    : ListView.builder(
                        itemCount: _fleet.length,
                        itemBuilder: (context, i) {
                          final f = _fleet[i];
                          final status = f.status == 'active'
                              ? 'Active'
                              : f.status == 'delivering'
                                  ? 'Delivers in ${f.availableIn} ticks'
                                  : f.status;
                          return ListTile(
                            dense: true,
                            title: Text(f.name, style: const TextStyle(color: Colors.white)),
                            subtitle: Text(
                              'Util ${f.util.toStringAsFixed(0)}% | $status',
                              style: const TextStyle(color: Colors.white70, fontSize: 12),
                            ),
                          );
                        },
                      ),
              ),
              const SizedBox(height: 4),
              Row(
                children: [
                  _buyButton('A320'),
                  const SizedBox(width: 8),
                  _buyButton('B738'),
                  const SizedBox(width: 8),
                  _buyButton('E190'),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _loadState() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.get(Uri.parse('http://localhost:4000/state'));
      if (resp.statusCode == 200) {
        final data = json.decode(resp.body) as Map<String, dynamic>;
        setState(() {
          _cash = (data['cash'] ?? 0).toDouble();
          _tick = data['tick'] ?? 0;
          final routes = (data['routes'] as List<dynamic>? ?? []);
          _routes = routes.map((e) => RouteInfo.fromJson(e as Map<String, dynamic>)).toList();
          final fleet = (data['fleet'] as List<dynamic>? ?? []);
          _fleet = fleet.map((e) => OwnedCraft.fromJson(e as Map<String, dynamic>)).toList();
        });
      } else {
        setState(() {
          _error = 'State load failed (${resp.statusCode})';
        });
      }
    } catch (e) {
      setState(() {
        _error = 'State load failed: $e';
      });
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _createRoute() async {
    if (_fromCtrl.text.isEmpty || _toCtrl.text.isEmpty) {
      setState(() => _error = 'Enter both airport codes');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final body = json.encode({
        'from': _fromCtrl.text.trim(),
        'to': _toCtrl.text.trim(),
        'aircraft_id': _aircraftId,
        'frequency_per_day': _freqPerDay,
        'one_way': _oneWay,
      });
      final resp = await http.post(
        Uri.parse('http://localhost:4000/routes'),
        headers: {'Content-Type': 'application/json'},
        body: body,
      );
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        final msg = resp.body.isNotEmpty ? resp.body : 'Create failed';
        setState(() => _error = '$msg (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Create failed: $e');
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _tickOnce() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.post(Uri.parse('http://localhost:4000/tick'));
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        setState(() => _error = 'Tick failed (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Tick failed: $e');
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Future<void> _buy(String templateId) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.post(
        Uri.parse('http://localhost:4000/fleet/purchase'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({'template_id': templateId}),
      );
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        final msg = resp.body.isNotEmpty ? resp.body : 'Purchase failed';
        setState(() => _error = '$msg (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Purchase failed: $e');
    } finally {
      if (mounted) {
        setState(() => _busy = false);
      }
    }
  }

  Widget _buyButton(String id) {
    final price = _prices[id];
    final lead = _lead[id];
    final label = price != null ? '\$${price ~/ 1_000_000}M' : '';
    final leadTxt = lead != null ? ' • ${lead}t lead' : '';
    return Expanded(
      child: ElevatedButton(
        onPressed: _busy ? null : () => _buy(id),
        child: Text('$id $label$leadTxt'),
      ),
    );
  }
}

class RouteInfo {
  RouteInfo({
    required this.id,
    required this.from,
    required this.to,
    required this.aircraftId,
    required this.freq,
    required this.price,
    required this.rev,
    required this.cost,
    required this.load,
    required this.profit,
    required this.blockMins,
    required this.landingFees,
    required this.curfewBlocked,
  });

  final String id;
  final String from;
  final String to;
  final String aircraftId;
  final int freq;
  final double price;
  final double rev;
  final double cost;
  final double load;
  final double profit;
  final double blockMins;
  final double landingFees;
  final bool curfewBlocked;

  factory RouteInfo.fromJson(Map<String, dynamic> json) {
    return RouteInfo(
      id: json['id'] ?? '',
      from: json['from'] ?? '',
      to: json['to'] ?? '',
      aircraftId: json['aircraft_id'] ?? '',
      freq: json['frequency_per_day'] ?? 0,
      price: (json['price_per_seat'] ?? 0).toDouble(),
      rev: (json['estimated_revenue_tick'] ?? 0).toDouble(),
      cost: (json['estimated_cost_tick'] ?? 0).toDouble(),
      load: (json['load_factor'] ?? 0).toDouble(),
      profit: (json['profit_per_tick'] ?? 0).toDouble(),
      blockMins: (json['block_minutes'] ?? 0).toDouble(),
      landingFees: (json['landing_fees_per_leg'] ?? 0).toDouble(),
      curfewBlocked: json['curfew_blocked'] ?? false,
    );
  }
}

class OwnedCraft {
  OwnedCraft({
    required this.id,
    required this.templateId,
    required this.name,
    required this.rangeKm,
    required this.seats,
    required this.cruiseKmh,
    required this.fuelCost,
    required this.turnaround,
    required this.status,
    required this.availableIn,
    required this.util,
  });

  final String id;
  final String templateId;
  final String name;
  final double rangeKm;
  final int seats;
  final double cruiseKmh;
  final double fuelCost;
  final int turnaround;
  final String status;
  final int availableIn;
  final double util;

  factory OwnedCraft.fromJson(Map<String, dynamic> json) {
    return OwnedCraft(
      id: json['id']?.toString() ?? '',
      templateId: json['template_id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      rangeKm: (json['range_km'] ?? 0).toDouble(),
      seats: json['seats'] ?? 0,
      cruiseKmh: (json['cruise_kmh'] ?? 0).toDouble(),
      fuelCost: (json['fuel_cost_per_km'] ?? 0).toDouble(),
      turnaround: json['turnaround_min'] ?? 0,
      status: json['status']?.toString() ?? '',
      availableIn: json['available_in_ticks'] ?? 0,
      util: (json['utilization_pct'] ?? 0).toDouble(),
    );
  }
}

class Airport {
  final String id;
  final String ident;
  final String type;
  final String name;
  final double lat;
  final double lon;
  final String country;
  final String region;
  final String city;
  final String iata;
  final String icao;

  Airport({
    required this.id,
    required this.ident,
    required this.type,
    required this.name,
    required this.lat,
    required this.lon,
    required this.country,
    required this.region,
    required this.city,
    required this.iata,
    required this.icao,
  });

  factory Airport.fromJson(Map<String, dynamic> json) {
    return Airport(
      id: json['id']?.toString() ?? '',
      ident: json['ident']?.toString() ?? '',
      type: json['type']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      lat: (json['lat'] as num?)?.toDouble() ?? 0.0,
      lon: (json['lon'] as num?)?.toDouble() ?? 0.0,
      country: json['country']?.toString() ?? '',
      region: json['region']?.toString() ?? '',
      city: json['city']?.toString() ?? '',
      iata: json['iata']?.toString() ?? '',
      icao: json['icao']?.toString() ?? '',
    );
  }
}
