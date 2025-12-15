import 'dart:convert';

import 'package:http/http.dart' as http;

import '../models/airport.dart';
import '../models/aircraft_template.dart';
import '../models/owned_craft.dart';
import '../models/route_analysis_result.dart';
import '../models/route_info.dart';

class GameSnapshot {
  GameSnapshot({
    required this.cash,
    required this.tick,
    required this.isRunning,
    required this.speed,
    required this.routes,
    required this.fleet,
  });

  final double cash;
  final int tick;
  final bool isRunning;
  final int speed;
  final List<RouteInfo> routes;
  final List<OwnedCraft> fleet;
}

class ApiService {
  ApiService({this.baseUrl = 'http://localhost:4000', http.Client? client})
    : _client = client ?? http.Client();

  final String baseUrl;
  final http.Client _client;

  Uri _url(String path) => Uri.parse('$baseUrl$path');
  String _errorMessage(http.Response resp, String fallback) {
    try {
      final decoded = json.decode(resp.body);
      if (decoded is Map<String, dynamic> && decoded['error'] is String) {
        return decoded['error'] as String;
      }
    } catch (_) {
      // ignore parse errors and use fallback
    }
    if (resp.body.isNotEmpty) {
      return resp.body;
    }
    return fallback;
  }

  Future<List<Airport>> fetchAirports({bool basic = false}) async {
    final query = basic ? '?tier=all&fields=basic' : '';
    final resp = await _client.get(_url('/airports$query'));
    if (resp.statusCode != 200) {
      throw Exception('Failed to load airports (${resp.statusCode})');
    }
    final data = json.decode(resp.body) as List<dynamic>;
    return data
        .map((e) => Airport.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<List<AircraftTemplate>> fetchAircraftTemplates() async {
    final resp = await _client.get(_url('/aircraft/templates'));
    if (resp.statusCode != 200) {
      throw Exception('Failed to load aircraft (${resp.statusCode})');
    }
    final data = json.decode(resp.body) as List<dynamic>;
    return data
        .map((e) => AircraftTemplate.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<GameSnapshot> fetchState() async {
    final resp = await _client.get(_url('/state'));
    if (resp.statusCode != 200) {
      throw Exception('Failed to load state (${resp.statusCode})');
    }
    final data = json.decode(resp.body) as Map<String, dynamic>;
    final routes = (data['routes'] as List<dynamic>? ?? [])
        .map((e) => RouteInfo.fromJson(e as Map<String, dynamic>))
        .toList();
    final fleet = (data['fleet'] as List<dynamic>? ?? [])
        .map((e) => OwnedCraft.fromJson(e as Map<String, dynamic>))
        .toList();
    return GameSnapshot(
      cash: (data['cash'] ?? 0).toDouble(),
      tick: data['tick'] ?? 0,
      isRunning: data['is_running'] ?? false,
      speed: data['speed'] ?? 1,
      routes: routes,
      fleet: fleet,
    );
  }

  Future<void> tick() async {
    final resp = await _client.post(_url('/tick'));
    if (resp.statusCode != 200) {
      throw Exception('Tick failed (${resp.statusCode})');
    }
  }

  Future<RouteInfo> createRoute({
    required String from,
    required String to,
    String via = '',
    required String aircraftId,
    required int frequency,
    required bool oneWay,
    required double userPrice,
  }) async {
    final resp = await _client.post(
      _url('/routes'),
      headers: {'Content-Type': 'application/json'},
      body: json.encode({
        'from': from,
        'to': to,
        'via': via,
        'aircraft_id': aircraftId,
        'frequency_per_day': frequency,
        'one_way': oneWay,
        'user_price': userPrice,
      }),
    );
    if (resp.statusCode != 200) {
      throw Exception(_errorMessage(resp, 'Create failed (${resp.statusCode})'));
    }
    return RouteInfo.fromJson(json.decode(resp.body) as Map<String, dynamic>);
  }

  Future<void> startSim(int speed) async {
    final resp = await _client.post(
      _url('/sim/start'),
      headers: {'Content-Type': 'application/json'},
      body: json.encode({'speed': speed}),
    );
    if (resp.statusCode != 200) {
      throw Exception('Start failed (${resp.statusCode})');
    }
  }

  Future<void> pauseSim() async {
    final resp = await _client.post(_url('/sim/pause'));
    if (resp.statusCode != 200) {
      throw Exception('Pause failed (${resp.statusCode})');
    }
  }

  Future<void> setSpeed(int speed) async {
    final resp = await _client.post(
      _url('/sim/speed'),
      headers: {'Content-Type': 'application/json'},
      body: json.encode({'speed': speed}),
    );
    if (resp.statusCode != 200) {
      throw Exception('Speed failed (${resp.statusCode})');
    }
  }

  Future<OwnedCraft> purchase(String templateId, String mode) async {
    final resp = await _client.post(
      _url('/fleet/purchase'),
      headers: {'Content-Type': 'application/json'},
      body: json.encode({'template_id': templateId, 'mode': mode}),
    );
    if (resp.statusCode != 200) {
      throw Exception(
        _errorMessage(resp, 'Purchase failed (${resp.statusCode})'),
      );
    }
    return OwnedCraft.fromJson(json.decode(resp.body) as Map<String, dynamic>);
  }

  Future<List<RouteAnalysisResult>> analyzeRoute({
    required String from,
    required String to,
    String via = '',
    required List<String> aircraftTypes,
  }) async {
    final resp = await _client.post(
      _url('/analysis/route'),
      headers: {'Content-Type': 'application/json'},
      body: json.encode({
        'origin': from,
        'dest': to,
        'via': via,
        'aircraft_types': aircraftTypes,
      }),
    );
    if (resp.statusCode != 200) {
      throw Exception('Analysis failed (${resp.statusCode})');
    }
    final data = json.decode(resp.body) as List<dynamic>;
    return data
        .map((e) => RouteAnalysisResult.fromJson(e as Map<String, dynamic>))
        .toList();
  }
}
