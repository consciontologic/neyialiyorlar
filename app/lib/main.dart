import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'screens/dashboard_screen.dart';

void main() {
  runApp(const ProviderScope(child: MyApp()));
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  // A single dark-first colour scheme keeps it sharp on any device.
  static const _seed = Color(0xFF1565C0);

  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'neyialiyorlar',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          useMaterial3: true,
          brightness: Brightness.light,
          colorScheme: ColorScheme.fromSeed(seedColor: _seed),
          inputDecorationTheme: const InputDecorationTheme(
            border: OutlineInputBorder(),
          ),
        ),
        darkTheme: ThemeData(
          useMaterial3: true,
          brightness: Brightness.dark,
          colorScheme: ColorScheme.fromSeed(
            seedColor: _seed,
            brightness: Brightness.dark,
          ),
          scaffoldBackgroundColor: const Color(0xFF0D1117),
          inputDecorationTheme: const InputDecorationTheme(
            border: OutlineInputBorder(),
          ),
        ),
        themeMode: ThemeMode.system,
        // SelectionArea makes every Text in the whole app copy-pastable on web.
        builder: (context, child) => SelectionArea(child: child!),
        home: const DashboardScreen(),
      );
}
