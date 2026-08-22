# React Native — Enterprise Grade

> **Version**: 1.0.0 | **Stack**: React Native 0.76, Expo SDK 52, TypeScript, Zustand

```tsx
// features/auth/LoginScreen.tsx
import { useState } from "react"
import { View, TextInput, Text, Pressable } from "react-native"
import * as SecureStore from "expo-secure-store"
import { api } from "@/lib/api"

export function LoginScreen() {
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")

  async function login() {
    setLoading(true); setError("")
    try {
      const { accessToken } = await api.post("/auth/login", { email, password })
      await SecureStore.setItemAsync("token", accessToken)
    } catch (e: any) {
      setError(e.message)
    } finally { setLoading(false) }
  }

  return (
    <View className="flex-1 justify-center p-6 gap-4">
      <TextInput value={email} onChangeText={setEmail} placeholder="Email" className="border rounded p-3" autoCapitalize="none" />
      <TextInput value={password} onChangeText={setPassword} placeholder="Password" secureTextEntry className="border rounded p-3" />
      {error ? <Text className="text-red-500">{error}</Text> : null}
      <Pressable onPress={login} disabled={loading} className="bg-blue-600 p-3 rounded">
        <Text className="text-white text-center">{loading ? "Loading..." : "Sign In"}</Text>
      </Pressable>
    </View>
  )
}
```

## Security: `expo-secure-store` (NOT AsyncStorage for tokens). CodePush disabled in production. Hermes engine enabled. `npx expo-doctor` for audits.
