# Flutter — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: Flutter 3.24+, Dart 3.5, Riverpod, Dio

```dart
// lib/features/auth/auth_screen.dart
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) => AuthNotifier());

class AuthState { final bool loading; final String? error; AuthState({this.loading = false, this.error}); }

class AuthNotifier extends StateNotifier<AuthState> {
  AuthNotifier() : super(AuthState());

  Future<void> login(String email, String password) async {
    state = AuthState(loading: true);
    try {
      final res = await Dio().post('/auth/login', data: {'email': email, 'password': password});
      // Store token securely — never SharedPreferences for secrets.
      await FlutterSecureStorage().write(key: 'token', value: res.data['accessToken']);
      state = AuthState();
    } catch (e) {
      state = AuthState(error: e.toString());
    }
  }
}

class LoginScreen extends ConsumerWidget {
  @override Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(authProvider);
    return Scaffold(
      body: Padding(padding: EdgeInsets.all(24), child: Column(children: [
        TextField(decoration: InputDecoration(labelText: 'Email')),
        TextField(decoration: InputDecoration(labelText: 'Password'), obscureText: true),
        if (state.error != null) Text(state.error!, style: TextStyle(color: Colors.red)),
        ElevatedButton(onPressed: state.loading ? null : () {}, child: state.loading ? CircularProgressIndicator() : Text('Sign In')),
      ])),
    );
  }
}
```

## Security: `flutter_secure_storage` for tokens. `dart pub outdated` for deps. Never hardcode API keys.
