<script setup lang="ts">
import { AuthForm, ForgotPasswordForm, PasswordLinkForm } from '@/features/auth'

// 公开认证页：登录、注册、找回密码，以及邮件链接落地的重置密码和账号恢复。
defineProps<{ mode: 'login' | 'register' | 'forgot' | 'reset' | 'recover' }>()
</script>

<template>
  <AuthForm v-if="mode === 'login' || mode === 'register'" :key="mode" :mode="mode" />
  <ForgotPasswordForm v-else-if="mode === 'forgot'" />
  <PasswordLinkForm v-else :key="mode" :purpose="mode" />
</template>
