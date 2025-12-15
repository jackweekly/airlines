import 'package:flutter/material.dart';

class SimControls extends StatelessWidget {
  const SimControls({
    super.key,
    required this.running,
    required this.busy,
    required this.speed,
    required this.onStart,
    required this.onPause,
    required this.onSetSpeed,
  });

  final bool running;
  final bool busy;
  final int speed;
  final void Function(int speed) onStart;
  final VoidCallback onPause;
  final void Function(int speed) onSetSpeed;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        IconButton(
          icon: Icon(
            running ? Icons.pause_circle_filled : Icons.play_circle_fill,
            color: Colors.white,
          ),
          onPressed: busy ? null : () => running ? onPause() : onStart(speed),
        ),
        Row(
          children: List.generate(4, (i) {
            final sp = i + 1;
            final active = sp == speed;
            return Padding(
              padding: const EdgeInsets.symmetric(horizontal: 2),
              child: GestureDetector(
                onTap: busy ? null : () => onSetSpeed(sp),
                child: Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 8,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: active
                        ? Colors.teal.withOpacity(0.25)
                        : Colors.white.withOpacity(0.08),
                    borderRadius: BorderRadius.circular(8),
                    border: Border.all(
                      color: active ? Colors.tealAccent : Colors.white24,
                    ),
                  ),
                  child: Text(
                    '${sp}x',
                    style: TextStyle(
                      color: active ? Colors.tealAccent : Colors.white,
                      fontWeight: active ? FontWeight.w700 : FontWeight.normal,
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
}
