<script setup lang="ts">
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import CardTitle from './ui/card/CardTitle.vue'
import Field from './ui/field/Field.vue'
import FieldDescription from './ui/field/FieldDescription.vue'
import FieldGroup from './ui/field/FieldGroup.vue'
import FieldLabel from './ui/field/FieldLabel.vue'
import Input from './ui/input/Input.vue'

defineProps<{ operations: boolean; registering: boolean; submitting: boolean; error: string }>()
const account = defineModel<string>('account', { required: true })
const password = defineModel<string>('password', { required: true })
const emit = defineEmits<{ submit: []; switchMode: [] }>()
</script>

<template>
  <!-- shadcn-vue login-03, with this console's supported sign-in methods. -->
  <Card class="border-solid">
    <CardHeader class="text-center">
      <CardTitle as="h1" class="text-xl tracking-normal">
        {{ operations ? '运营登录' : registering ? '注册代理控制台' : '登录控制台' }}
      </CardTitle>
    </CardHeader>
    <CardContent>
      <form @submit.prevent="emit('submit')">
        <FieldGroup>
          <Field>
            <FieldLabel for="login-account">{{ operations ? '运营账号' : '手机号' }}</FieldLabel>
            <Input id="login-account" v-model="account" :type="operations ? 'text' : 'tel'"
              :placeholder="operations ? undefined : '可带 +86'" autocomplete="username" class="min-h-0" required />
          </Field>
          <Field>
            <FieldLabel for="login-password">密码</FieldLabel>
            <Input id="login-password" v-model="password" type="password"
              :autocomplete="registering ? 'new-password' : 'current-password'"
              :minlength="registering ? 12 : undefined" class="min-h-0" required />
            <FieldDescription v-if="!operations && !registering">忘记密码？请联系运营核实租户归属后重置。</FieldDescription>
          </Field>
          <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
          <Field>
            <Button type="submit" class="min-h-0 border-0" :disabled="submitting">
              {{ submitting ? '正在提交…' : registering ? '注册' : '登录' }}
            </Button>
            <FieldDescription v-if="!operations" class="text-center">
              <Button variant="link" type="button" class="h-auto min-h-0 border-0 bg-transparent p-0 hover:bg-transparent"
                :disabled="submitting" @click="emit('switchMode')">
                {{ registering ? '已有账号，去登录' : '注册账号' }}
              </Button>
            </FieldDescription>
          </Field>
        </FieldGroup>
      </form>
    </CardContent>
  </Card>
</template>
