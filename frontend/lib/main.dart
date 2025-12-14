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
                        padding: EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 8,
                        ),
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
                        padding: const EdgeInsets.symmetric(
                          horizontal: 12,
                          vertical: 8,
                        ),
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
                    const Text(
                      'Settings',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 16,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    IconButton(
                      onPressed: () => setState(() => _showSettings = false),
                      icon: const Icon(Icons.close, color: Colors.white),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
                const Text(
                  'Theme / Style',
                  style: TextStyle(color: Colors.white70),
                ),
                DropdownButtonHideUnderline(
                  child: DropdownButton<String>(
                    dropdownColor: Colors.black87,
                    value: _currentStyle,
                    items: _mapboxStyles.keys
                        .map(
                          (name) => DropdownMenuItem(
                            value: name,
                            child: Text(
                              name,
                              style: const TextStyle(color: Colors.white),
                            ),
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
        (a) => a.type == 'large_airport' || a.type == 'medium_airport',
      );
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

enum RouteFieldRole { from, via, to }

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
  final TextEditingController _viaCtrl = TextEditingController();
  final TextEditingController _toCtrl = TextEditingController();
  String _aircraftId = 'A320';
  int _freqPerDay = 1;
  bool _busy = false;
  String? _error;
  final Map<String, double> _prices = const {
    'ATR72': 26_000_000,
    'CRJ9': 44_000_000,
    'E175': 48_000_000,
    'E190': 52_000_000,
    'E195E2': 60_000_000,
    'B737-700': 82_000_000,
    'B737-800': 96_000_000,
    'B737MAX8': 120_000_000,
    'A320': 98_000_000,
    'A320NEO': 110_000_000,
    'A321NEO': 125_000_000,
    'B767-300ER': 220_000_000,
    'B777-300ER': 375_000_000,
    'B787-9': 292_000_000,
    'A330-900': 296_000_000,
    'A350-900': 317_000_000,
    'B747-400': 250_000_000,
    'A380-800': 445_000_000,
  };
  final Map<String, int> _lead = const {
    'ATR72': 5,
    'CRJ9': 6,
    'E175': 6,
    'E190': 6,
    'E195E2': 7,
    'B737-700': 7,
    'B737-800': 8,
    'B737MAX8': 8,
    'A320': 8,
    'A320NEO': 8,
    'A321NEO': 9,
    'B767-300ER': 10,
    'B777-300ER': 11,
    'B787-9': 10,
    'A330-900': 10,
    'A350-900': 11,
    'B747-400': 12,
    'A380-800': 12,
  };
  bool _oneWay = false;
  String _activePanel = ''; // '', 'routes', 'fleet'
  bool _running = false;
  int _simSpeed = 1;
  bool _pickingFrom = false;
  bool _pickingVia = false;
  bool _pickingTo = false;
  List<Airport> _airportList = const [];
  List<AircraftTemplate> _templates = const [];
  bool _loadingTemplates = false;
  String? _templatesError;
  void _notifySelection() {
    final msg = {
      'type': 'set_selection',
      'from': _fromCtrl.text.trim(),
      'via': _viaCtrl.text.trim(),
      'to': _toCtrl.text.trim(),
    };
    _iframe.contentWindow?.postMessage(msg, '*');
  }

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
    ui_web.platformViewRegistry.registerViewFactory(
      _viewId,
      (int _) => _iframe,
    );

    _loadAirportsList();
    _loadAircraftTemplates();

    html.window.onMessage.listen((event) {
      final data = event.data;
      if (data is Map && data['type'] == 'airport_select') {
        final ident = (data['ident'] ?? '').toString();
        if (ident.isEmpty) return;
        if (_pickingFrom) {
          setState(() {
            _fromCtrl.text = ident;
            _pickingFrom = false;
          });
          _notifySelection();
        } else if (_pickingVia) {
          setState(() {
            _viaCtrl.text = ident;
            _pickingVia = false;
          });
          _notifySelection();
        } else if (_pickingTo) {
          setState(() {
            _toCtrl.text = ident;
            _pickingTo = false;
          });
          _notifySelection();
        } else {
          // default to fill "from" if neither pick mode is active
          setState(() {
            _fromCtrl.text = ident;
          });
          _notifySelection();
        }
      }
    });
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
                      padding: EdgeInsets.symmetric(
                        horizontal: 12,
                        vertical: 8,
                      ),
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
                top: 12,
                left: 12,
                right: 12,
                child: PointerInterceptor(child: _topBar()),
              ),
              Positioned(
                left: 12,
                right: 12,
                bottom: 12,
                child: PointerInterceptor(child: _bottomBar()),
              ),
              if (_activePanel.isNotEmpty)
                Positioned(
                  left: 12,
                  right: 12,
                  bottom: 76,
                  child: PointerInterceptor(child: _floatingPanel()),
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
                      const Text(
                        'Settings',
                        style: TextStyle(
                          color: Colors.white,
                          fontSize: 16,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                      IconButton(
                        onPressed: () => setState(() => _showSettings = false),
                        icon: const Icon(Icons.close, color: Colors.white),
                      ),
                    ],
                  ),
                  const SizedBox(height: 8),
                  const Text(
                    'Theme / Style',
                    style: TextStyle(color: Colors.white70),
                  ),
                  const SizedBox(height: 8),
                  ..._mapboxStyles.entries.map((entry) {
                    final isSelected = entry.value == _currentStyle;
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      dense: true,
                      leading: Icon(
                        isSelected
                            ? Icons.radio_button_checked
                            : Icons.radio_button_unchecked,
                        color: Colors.white,
                      ),
                      title: Text(
                        entry.key,
                        style: const TextStyle(color: Colors.white),
                      ),
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
        .firstWhere(
          (e) => e.value == styleId,
          orElse: () => _mapboxStyles.entries.first,
        )
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

  Widget _topBar() {
    return Material(
      color: Colors.black.withOpacity(0.75),
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
        child: Row(
          children: [
            const Text(
              'Airline Builder',
              style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w700,
              ),
            ),
            const SizedBox(width: 12),
            _kpiRow(),
            const Spacer(),
            _simControls(),
            const SizedBox(width: 8),
            OutlinedButton(
              onPressed: _busy ? null : _tickOnce,
              style: OutlinedButton.styleFrom(
                side: const BorderSide(color: Colors.white24),
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 10,
                ),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: const Text('Advance tick'),
            ),
            const SizedBox(width: 8),
            IconButton(
              icon: const Icon(Icons.refresh, color: Colors.white70),
              onPressed: _busy ? null : _loadState,
            ),
          ],
        ),
      ),
    );
  }

  Widget _bottomBar() {
    final buttons = _templates
        .map(
          (tpl) => Padding(
            padding: const EdgeInsets.only(right: 8),
            child: _buyButton(tpl),
          ),
        )
        .toList();
    return Material(
      color: Colors.black.withOpacity(0.75),
      borderRadius: BorderRadius.circular(12),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        child: Row(
          children: [
            _bottomChip('Routes', Icons.add, 'routes'),
            const SizedBox(width: 8),
            _bottomChip('Fleet', Icons.flight_takeoff, 'fleet'),
            const SizedBox(width: 12),
            Expanded(
              child: _templates.isEmpty
                  ? Align(
                      alignment: Alignment.centerRight,
                      child: Text(
                        _templatesError ??
                            (_loadingTemplates
                                ? 'Loading aircraft…'
                                : 'No aircraft available'),
                        style: const TextStyle(color: Colors.white70),
                      ),
                    )
                  : SingleChildScrollView(
                      scrollDirection: Axis.horizontal,
                      reverse: true,
                      child: Row(
                        mainAxisAlignment: MainAxisAlignment.end,
                        children: buttons,
                      ),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _bottomChip(String label, IconData icon, String key) {
    final active = _activePanel == key;
    return GestureDetector(
      onTap: () => setState(() => _activePanel = active ? '' : key),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: active
              ? Colors.teal.withOpacity(0.2)
              : Colors.white.withOpacity(0.06),
          borderRadius: BorderRadius.circular(10),
          border: Border.all(
            color: active ? Colors.tealAccent : Colors.white24,
          ),
        ),
        child: Row(
          children: [
            Icon(icon, color: Colors.white, size: 16),
            const SizedBox(width: 6),
            Text(label, style: const TextStyle(color: Colors.white)),
          ],
        ),
      ),
    );
  }

  Widget _floatingPanel() {
    final isRoutes = _activePanel == 'routes';
    final height = isRoutes ? 200.0 : 150.0;
    final width = 360.0;
    return Align(
      alignment: Alignment.bottomLeft,
      child: Material(
        color: Colors.black.withOpacity(0.85),
        borderRadius: BorderRadius.circular(14),
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: height, maxWidth: width),
          child: Padding(
            padding: const EdgeInsets.all(10),
            child: SingleChildScrollView(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                mainAxisSize: MainAxisSize.min,
                children: [
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        isRoutes ? 'Routes' : 'Fleet',
                        style: const TextStyle(
                          color: Colors.white,
                          fontWeight: FontWeight.w700,
                          fontSize: 14,
                        ),
                      ),
                      IconButton(
                        icon: const Icon(Icons.close, color: Colors.white70),
                        onPressed: () => setState(() => _activePanel = ''),
                      ),
                    ],
                  ),
                  const SizedBox(height: 6),
                  if (isRoutes) ...[
                    _routeForm(),
                    const SizedBox(height: 8),
                    _ctaRow(),
                    if (_error != null)
                      Padding(
                        padding: const EdgeInsets.only(top: 4),
                        child: Text(
                          _error!,
                          style: const TextStyle(
                            color: Colors.redAccent,
                            fontSize: 12,
                          ),
                        ),
                      ),
                    const SizedBox(height: 8),
                    _sectionTitle('Active routes'),
                    SizedBox(
                      height: 90,
                      child: _routes.isEmpty
                          ? const Center(
                              child: Text(
                                'No routes yet',
                                style: TextStyle(color: Colors.white54),
                              ),
                            )
                          : ListView.builder(
                              itemCount: _routes.length,
                              itemBuilder: (context, i) =>
                                  _routeTile(_routes[i]),
                            ),
                    ),
                  ] else ...[
                    _sectionTitle('Fleet'),
                    SizedBox(
                      height: 90,
                      child: _fleet.isEmpty
                          ? const Center(
                              child: Text(
                                'No aircraft',
                                style: TextStyle(color: Colors.white54),
                              ),
                            )
                          : ListView.builder(
                              itemCount: _fleet.length,
                              itemBuilder: (context, i) =>
                                  _fleetTile(_fleet[i]),
                            ),
                    ),
                  ],
                ],
              ),
            ),
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
          _running = data['is_running'] ?? false;
          _simSpeed = data['speed'] ?? 1;
          final routes = (data['routes'] as List<dynamic>? ?? []);
          _routes = routes
              .map((e) => RouteInfo.fromJson(e as Map<String, dynamic>))
              .toList();
          final fleet = (data['fleet'] as List<dynamic>? ?? []);
          _fleet = fleet
              .map((e) => OwnedCraft.fromJson(e as Map<String, dynamic>))
              .toList();
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
        'via': _viaCtrl.text.trim(),
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

  Future<void> _loadAirportsList() async {
    try {
      final resp = await http.get(
        Uri.parse('http://localhost:4000/airports?tier=medium&fields=basic'),
      );
      if (resp.statusCode == 200) {
        final data = json.decode(resp.body) as List<dynamic>;
        final list = data
            .map((e) => Airport.fromJson(e as Map<String, dynamic>))
            .where((a) => a.ident.isNotEmpty)
            .toList();
        setState(() {
          _airportList = list;
        });
      }
    } catch (_) {
      // ignore for now; fallback to manual entry
    }
  }

  Future<void> _loadAircraftTemplates() async {
    setState(() {
      _loadingTemplates = true;
      _templatesError = null;
    });
    try {
      final resp = await http.get(
        Uri.parse('http://localhost:4000/aircraft/templates'),
      );
      if (resp.statusCode == 200) {
        final data = json.decode(resp.body) as List<dynamic>;
        final list = data
            .map((e) => AircraftTemplate.fromJson(e as Map<String, dynamic>))
            .toList();
        setState(() {
          _templates = list;
          if (list.isNotEmpty && !list.any((t) => t.id == _aircraftId)) {
            _aircraftId = list.first.id;
          }
        });
      } else {
        setState(
          () =>
              _templatesError = 'Failed to load aircraft (${resp.statusCode})',
        );
      }
    } catch (e) {
      setState(() => _templatesError = 'Failed to load aircraft: $e');
    } finally {
      if (mounted) {
        setState(() => _loadingTemplates = false);
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

  Future<void> _startSim(int speed) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.post(
        Uri.parse('http://localhost:4000/sim/start'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({'speed': speed}),
      );
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        setState(() => _error = 'Start failed (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Start failed: $e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pauseSim() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.post(
        Uri.parse('http://localhost:4000/sim/pause'),
      );
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        setState(() => _error = 'Pause failed (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Pause failed: $e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _setSimSpeed(int speed) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final resp = await http.post(
        Uri.parse('http://localhost:4000/sim/speed'),
        headers: {'Content-Type': 'application/json'},
        body: json.encode({'speed': speed}),
      );
      if (resp.statusCode == 200) {
        await _loadState();
      } else {
        setState(() => _error = 'Speed failed (${resp.statusCode})');
      }
    } catch (e) {
      setState(() => _error = 'Speed failed: $e');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Widget _buyButton(AircraftTemplate template) {
    final id = template.id;
    final price = _prices[id];
    final lead = _lead[id];
    final label = price != null ? '\$${price ~/ 1_000_000}M' : '';
    final leadTxt = lead != null ? ' • ${lead}t lead' : '';
    return ElevatedButton(
      onPressed: _busy ? null : () => _buy(id),
      style: ElevatedButton.styleFrom(
        backgroundColor: Colors.white.withOpacity(0.1),
        foregroundColor: Colors.white,
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(10)),
      ),
      child: Text('$id $label$leadTxt'),
    );
  }

  Widget _panelHeader() {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: const [
            Text(
              'Routes',
              style: TextStyle(
                color: Colors.white,
                fontWeight: FontWeight.w700,
                fontSize: 16,
              ),
            ),
          ],
        ),
        IconButton(
          icon: const Icon(Icons.refresh, color: Colors.white70, size: 18),
          onPressed: _busy ? null : _loadState,
          padding: EdgeInsets.zero,
          constraints: const BoxConstraints(),
        ),
      ],
    );
  }

  Widget _kpiRow() {
    final cards = [
      _kpiChip('Cash', '\$${_cash.toStringAsFixed(0)}'),
      _kpiChip('Tick', '$_tick'),
      if (_routes.isNotEmpty)
        _kpiChip(
          'Avg load',
          '${(_routes.map((e) => e.load).fold<double>(0, (a, b) => a + b) / _routes.length * 100).toStringAsFixed(0)}%',
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

  Widget _kpiChip(String label, String value) {
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
            style: const TextStyle(
              color: Colors.white,
              fontWeight: FontWeight.w600,
              fontSize: 13,
            ),
          ),
        ],
      ),
    );
  }

  Widget _simControls() {
    return Row(
      children: [
        IconButton(
          icon: Icon(
            _running ? Icons.pause_circle_filled : Icons.play_circle_fill,
            color: Colors.white,
          ),
          onPressed: _busy
              ? null
              : () => _running ? _pauseSim() : _startSim(_simSpeed),
        ),
        Row(
          children: List.generate(4, (i) {
            final sp = i + 1;
            final active = sp == _simSpeed;
            return Padding(
              padding: const EdgeInsets.symmetric(horizontal: 2),
              child: GestureDetector(
                onTap: _busy ? null : () => _setSimSpeed(sp),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: active
                        ? Colors.teal.withOpacity(0.2)
                        : Colors.white.withOpacity(0.06),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: active ? Colors.tealAccent : Colors.white24,
                    ),
                  ),
                  child: Text(
                    '${sp}x',
                    style: TextStyle(
                      color: Colors.white,
                      fontSize: 12,
                      fontWeight: active ? FontWeight.w700 : FontWeight.w500,
                    ),
                  ),
                ),
              ),
            );
          }),
        ),
      ],
    );
  }

  Widget _routeForm() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: _airportAutocomplete(
                _fromCtrl,
                'From (IATA/ICAO)',
                RouteFieldRole.from,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: _airportAutocomplete(
                _viaCtrl,
                'Via / Stopover (optional)',
                RouteFieldRole.via,
              ),
            ),
            const SizedBox(width: 8),
            Expanded(
              child: _airportAutocomplete(_toCtrl, 'To', RouteFieldRole.to),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Row(
          children: [
            Expanded(
              child: DropdownButtonFormField<String>(
                value: _templates.any((t) => t.id == _aircraftId)
                    ? _aircraftId
                    : (_templates.isNotEmpty ? _templates.first.id : null),
                dropdownColor: Colors.black87,
                decoration: _inputDecoration('Aircraft', null),
                items: _templates
                    .map(
                      (tpl) => DropdownMenuItem(
                        value: tpl.id,
                        child: Text(
                          '${tpl.name} (${tpl.seats} seats)',
                          style: const TextStyle(color: Colors.white),
                        ),
                      ),
                    )
                    .toList(),
                hint: const Text(
                  'Select aircraft',
                  style: TextStyle(color: Colors.white),
                ),
                onChanged: _templates.isEmpty
                    ? null
                    : (v) {
                        if (v == null) return;
                        setState(() => _aircraftId = v);
                      },
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    'Round Trips/day',
                    style: TextStyle(color: Colors.white70, fontSize: 12),
                  ),
                  Slider(
                    value: _freqPerDay.toDouble(),
                    min: 1,
                    max: 10,
                    divisions: 9,
                    activeColor: Colors.tealAccent,
                    inactiveColor: Colors.white24,
                    label: '$_freqPerDay',
                    onChanged: (v) => setState(() => _freqPerDay = v.toInt()),
                  ),
                ],
              ),
            ),
          ],
        ),
        const SizedBox(height: 4),
        Row(
          children: [
            TextButton(
              onPressed: () => setState(() {
                _pickingFrom = true;
                _pickingVia = false;
                _pickingTo = false;
              }),
              style: TextButton.styleFrom(
                backgroundColor: _pickingFrom
                    ? Colors.teal.withOpacity(0.2)
                    : Colors.transparent,
                side: BorderSide(
                  color: _pickingFrom ? Colors.tealAccent : Colors.white24,
                ),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: Text(
                _pickingFrom ? 'Picking From… (click map)' : 'Pick From',
                style: const TextStyle(color: Colors.white),
              ),
            ),
            const SizedBox(width: 6),
            TextButton(
              onPressed: () => setState(() {
                _pickingVia = true;
                _pickingFrom = false;
                _pickingTo = false;
              }),
              style: TextButton.styleFrom(
                backgroundColor: _pickingVia
                    ? Colors.teal.withOpacity(0.2)
                    : Colors.transparent,
                side: BorderSide(
                  color: _pickingVia ? Colors.tealAccent : Colors.white24,
                ),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: Text(
                _pickingVia ? 'Picking Via… (click map)' : 'Pick Via',
                style: const TextStyle(color: Colors.white),
              ),
            ),
            const SizedBox(width: 6),
            TextButton(
              onPressed: () => setState(() {
                _pickingTo = true;
                _pickingFrom = false;
                _pickingVia = false;
              }),
              style: TextButton.styleFrom(
                backgroundColor: _pickingTo
                    ? Colors.teal.withOpacity(0.2)
                    : Colors.transparent,
                side: BorderSide(
                  color: _pickingTo ? Colors.tealAccent : Colors.white24,
                ),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
              child: Text(
                _pickingTo ? 'Picking To… (click map)' : 'Pick To',
                style: const TextStyle(color: Colors.white),
              ),
            ),
            const Spacer(),
            IconButton(
              icon: const Icon(Icons.swap_horiz, color: Colors.white70),
              onPressed: () {
                final tmp = _fromCtrl.text;
                setState(() {
                  _fromCtrl.text = _toCtrl.text;
                  _toCtrl.text = tmp;
                });
                _notifySelection();
              },
            ),
          ],
        ),
        const SizedBox(height: 4),
        GestureDetector(
          onTap: () => setState(() => _oneWay = !_oneWay),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
            decoration: BoxDecoration(
              color: _oneWay
                  ? Colors.teal.withOpacity(0.2)
                  : Colors.white.withOpacity(0.06),
              borderRadius: BorderRadius.circular(10),
              border: Border.all(
                color: _oneWay ? Colors.tealAccent : Colors.white24,
              ),
            ),
            child: Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text(
                  'One-way only',
                  style: TextStyle(color: Colors.white70),
                ),
                Switch(
                  value: _oneWay,
                  onChanged: (v) => setState(() => _oneWay = v),
                  activeColor: Colors.tealAccent,
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  InputDecoration _inputDecoration(String label, String? hint) {
    return InputDecoration(
      labelText: label,
      hintText: hint,
      labelStyle: const TextStyle(color: Colors.white70),
      hintStyle: const TextStyle(color: Colors.white38),
      filled: true,
      fillColor: Colors.white.withOpacity(0.06),
      enabledBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: Colors.white24),
      ),
      focusedBorder: OutlineInputBorder(
        borderRadius: BorderRadius.circular(10),
        borderSide: const BorderSide(color: Colors.tealAccent),
      ),
      contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
    );
  }

  Widget _airportAutocomplete(
    TextEditingController controller,
    String label,
    RouteFieldRole role,
  ) {
    return Autocomplete<Airport>(
      optionsBuilder: (text) {
        final q = text.text.toLowerCase();
        if (q.isEmpty) return const Iterable<Airport>.empty();
        return _airportList.where((a) {
          return a.ident.toLowerCase().contains(q) ||
              a.name.toLowerCase().contains(q) ||
              a.city.toLowerCase().contains(q);
        });
      },
      displayStringForOption: (a) =>
          '${a.ident} - ${a.name}${a.city.isNotEmpty ? ' (${a.city})' : ''}',
      fieldViewBuilder: (context, textCtrl, focusNode, onFieldSubmitted) {
        textCtrl.text = controller.text;
        textCtrl.selection = TextSelection.collapsed(
          offset: textCtrl.text.length,
        );
        return TextField(
          controller: textCtrl,
          focusNode: focusNode,
          decoration: _inputDecoration(label, 'Search by code, name, city'),
          style: const TextStyle(color: Colors.white),
          onChanged: (val) {
            controller.text = val;
            _notifySelection();
          },
        );
      },
      onSelected: (a) {
        setState(() {
          controller.text = a.ident;
          switch (role) {
            case RouteFieldRole.from:
              _pickingFrom = false;
              break;
            case RouteFieldRole.via:
              _pickingVia = false;
              break;
            case RouteFieldRole.to:
              _pickingTo = false;
              break;
          }
        });
        _notifySelection();
      },
      optionsViewBuilder: (context, onSelected, options) {
        return Align(
          alignment: Alignment.topLeft,
          child: Material(
            color: Colors.black87,
            borderRadius: BorderRadius.circular(8),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 220, maxWidth: 320),
              child: ListView.builder(
                padding: EdgeInsets.zero,
                itemCount: options.length,
                itemBuilder: (context, i) {
                  final a = options.elementAt(i);
                  return ListTile(
                    dense: true,
                    title: Text(
                      '${a.ident} - ${a.name}',
                      style: const TextStyle(color: Colors.white),
                    ),
                    subtitle: Text(
                      a.city,
                      style: const TextStyle(
                        color: Colors.white70,
                        fontSize: 12,
                      ),
                    ),
                    onTap: () => onSelected(a),
                  );
                },
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _ctaRow() {
    return Row(
      children: [
        Expanded(
          child: ElevatedButton(
            onPressed: _busy ? null : _createRoute,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.tealAccent,
              foregroundColor: Colors.black87,
              padding: const EdgeInsets.symmetric(vertical: 12),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(10),
              ),
            ),
            child: _busy
                ? const SizedBox(
                    height: 16,
                    width: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Text('Create route'),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: OutlinedButton(
            onPressed: _busy ? null : _tickOnce,
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: Colors.white24),
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 12),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(10),
              ),
            ),
            child: const Text('Advance tick'),
          ),
        ),
      ],
    );
  }

  Widget _sectionTitle(String title) {
    return Text(
      title,
      style: const TextStyle(
        color: Colors.white70,
        fontWeight: FontWeight.w700,
      ),
    );
  }

  Widget _routeTile(RouteInfo r) {
    final profitPos = r.profit >= 0;
    final routeLabel = r.via.isNotEmpty
        ? '${r.from} → ${r.to} (via ${r.via})'
        : '${r.from} → ${r.to}';
    final loadForDisplay = r.lastLoad > 0 ? r.lastLoad : r.load;
    final revForDisplay = r.lastRev > 0 ? r.lastRev : r.rev;
    return Container(
      margin: const EdgeInsets.only(bottom: 4),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white.withOpacity(0.04),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.white24),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Expanded(
                child: Text(
                  routeLabel,
                  style: const TextStyle(
                    color: Colors.white,
                    fontWeight: FontWeight.w600,
                    fontSize: 13,
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const SizedBox(width: 6),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: profitPos
                      ? Colors.teal.withOpacity(0.2)
                      : Colors.red.withOpacity(0.2),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  r.aircraftId,
                  style: TextStyle(
                    color: profitPos ? Colors.tealAccent : Colors.redAccent,
                    fontSize: 11,
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            'Freq ${r.freq}/d • Block ${r.blockMins.toStringAsFixed(0)}m • Last Load ${(loadForDisplay * 100).clamp(0, 999).toStringAsFixed(0)}% • Fees \$${r.landingFees.toStringAsFixed(0)}/leg',
            style: const TextStyle(color: Colors.white70, fontSize: 11),
          ),
          Text(
            'Last Rev \$${revForDisplay.toStringAsFixed(0)} • Cost \$${r.cost.toStringAsFixed(0)} • Profit \$${r.profit.toStringAsFixed(0)}${r.curfewBlocked ? ' • Curfew blocked' : ''}',
            style: const TextStyle(color: Colors.white70, fontSize: 11),
          ),
        ],
      ),
    );
  }

  Widget _fleetTile(OwnedCraft f) {
    final status = f.status == 'active'
        ? 'Active'
        : f.status == 'delivering'
        ? 'Delivers in ${f.availableIn} ticks'
        : f.status;
    final isMaintenance = f.status.toLowerCase() == 'maintenance';
    final conditionColor =
        Color.lerp(
          Colors.redAccent,
          Colors.tealAccent,
          (f.condition / 100).clamp(0.0, 1.0),
        ) ??
        Colors.tealAccent;
    return Container(
      margin: const EdgeInsets.only(bottom: 4),
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
      decoration: BoxDecoration(
        color: Colors.white.withOpacity(0.04),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: Colors.white24),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                f.name,
                style: const TextStyle(
                  color: Colors.white,
                  fontWeight: FontWeight.w600,
                  fontSize: 13,
                ),
              ),
              Text(
                status,
                style: TextStyle(
                  color: isMaintenance ? Colors.redAccent : Colors.white70,
                  fontSize: 11,
                  fontWeight: isMaintenance ? FontWeight.w600 : FontWeight.w400,
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Stack(
            children: [
              Container(
                height: 6,
                decoration: BoxDecoration(
                  color: Colors.white12,
                  borderRadius: BorderRadius.circular(6),
                ),
              ),
              FractionallySizedBox(
                widthFactor: (f.util / 100).clamp(0.0, 1.0),
                child: Container(
                  height: 6,
                  decoration: BoxDecoration(
                    color: Colors.tealAccent,
                    borderRadius: BorderRadius.circular(6),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Stack(
            children: [
              Container(
                height: 6,
                decoration: BoxDecoration(
                  color: Colors.white12,
                  borderRadius: BorderRadius.circular(6),
                ),
              ),
              FractionallySizedBox(
                widthFactor: (f.condition / 100).clamp(0.0, 1.0),
                child: Container(
                  height: 6,
                  decoration: BoxDecoration(
                    color: conditionColor,
                    borderRadius: BorderRadius.circular(6),
                  ),
                ),
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text(
            'Util ${f.util.toStringAsFixed(0)}%',
            style: const TextStyle(color: Colors.white70, fontSize: 11),
          ),
          Text(
            'Condition ${f.condition.toStringAsFixed(0)}%',
            style: const TextStyle(color: Colors.white70, fontSize: 11),
          ),
        ],
      ),
    );
  }
}

class RouteInfo {
  RouteInfo({
    required this.id,
    required this.from,
    required this.to,
    required this.via,
    required this.aircraftId,
    required this.freq,
    required this.price,
    required this.rev,
    required this.cost,
    required this.load,
    required this.lastRev,
    required this.lastLoad,
    required this.profit,
    required this.blockMins,
    required this.landingFees,
    required this.curfewBlocked,
  });

  final String id;
  final String from;
  final String to;
  final String via;
  final String aircraftId;
  final int freq;
  final double price;
  final double rev;
  final double cost;
  final double load;
  final double lastRev;
  final double lastLoad;
  final double profit;
  final double blockMins;
  final double landingFees;
  final bool curfewBlocked;

  factory RouteInfo.fromJson(Map<String, dynamic> json) {
    return RouteInfo(
      id: json['id'] ?? '',
      from: json['from'] ?? '',
      to: json['to'] ?? '',
      via: json['via']?.toString() ?? '',
      aircraftId: json['aircraft_id'] ?? '',
      freq: json['frequency_per_day'] ?? 0,
      price: (json['price_per_seat'] ?? 0).toDouble(),
      rev: (json['estimated_revenue_tick'] ?? 0).toDouble(),
      cost: (json['estimated_cost_tick'] ?? 0).toDouble(),
      load: (json['load_factor'] ?? 0).toDouble(),
      lastRev: (json['last_tick_revenue'] ?? 0).toDouble(),
      lastLoad: (json['last_tick_load'] ?? 0).toDouble(),
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
    required this.condition,
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
  final double condition;

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
      condition: (json['condition_pct'] ?? 0).toDouble(),
    );
  }
}

class AircraftTemplate {
  AircraftTemplate({
    required this.id,
    required this.name,
    required this.rangeKm,
    required this.seats,
    required this.cruiseKmh,
    required this.fuelCostPerKm,
    required this.turnaroundMin,
  });

  final String id;
  final String name;
  final double rangeKm;
  final int seats;
  final double cruiseKmh;
  final double fuelCostPerKm;
  final int turnaroundMin;

  factory AircraftTemplate.fromJson(Map<String, dynamic> json) {
    return AircraftTemplate(
      id: json['id']?.toString() ?? '',
      name: json['name']?.toString() ?? '',
      rangeKm: (json['range_km'] ?? 0).toDouble(),
      seats: json['seats'] ?? 0,
      cruiseKmh: (json['cruise_kmh'] ?? 0).toDouble(),
      fuelCostPerKm: (json['fuel_cost_per_km'] ?? 0).toDouble(),
      turnaroundMin: json['turnaround_min'] ?? 0,
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
