<script setup lang="ts">
import { ref, watch } from 'vue'
import { Eye, EyeOff } from 'lucide-vue-next'
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

const props = defineProps<{ operations: boolean; registering: boolean; submitting: boolean; error: string }>()
const account = defineModel<string>('account', { required: true })
const password = defineModel<string>('password', { required: true })
const passwordVisible = ref(false)
const googleDemoSelected = ref(false)
watch(() => props.registering, () => { googleDemoSelected.value = false })
watch(password, value => {
  if (!value) passwordVisible.value = false
})
const emit = defineEmits<{ submit: []; switchMode: [] }>()
</script>

<template>
  <Card class="auth-form-card">
    <CardHeader>
      <CardTitle as="h1" class="tracking-normal">
        {{ operations ? '登录运营控制台' : registering ? '创建一个账户' : '登录您的账户' }}
      </CardTitle>
    </CardHeader>
    <CardContent>
      <div v-if="!operations" class="mb-5 grid gap-3">
        <Button variant="outline" type="button" class="google-demo-button w-full" @click="googleDemoSelected = true">
          <span class="google-mark" aria-hidden="true">G</span>
          {{ registering ? '使用 Google 账号注册' : '使用 Google 账号登录' }}
        </Button>
        <p v-if="googleDemoSelected" class="text-sm text-muted-foreground" role="status">
          Google 按钮仅供演示，请使用手机号继续。
        </p>
        <p class="text-center text-sm text-muted-foreground" aria-hidden="true">或</p>
      </div>
      <form @submit.prevent="emit('submit')">
        <FieldGroup>
          <Field>
            <FieldLabel for="login-account">{{ operations ? '运营账号' : '手机号' }}</FieldLabel>
            <Input id="login-account" v-model="account" :type="operations ? 'text' : 'tel'"
              :placeholder="operations ? undefined : '11 位手机号，可带 +86'" autocomplete="username" class="min-h-0" required />
          </Field>
          <Field>
            <FieldLabel for="login-password">密码</FieldLabel>
            <div class="login-password">
              <Input id="login-password" v-model="password" :type="passwordVisible ? 'text' : 'password'"
                :autocomplete="registering ? 'new-password' : 'current-password'"
                :minlength="registering ? 12 : undefined" :aria-describedby="registering ? 'registration-password-rule' : undefined"
                class="min-h-0 w-full pr-12" required />
              <Button variant="ghost" type="button" :aria-label="passwordVisible ? '隐藏密码' : '显示密码'"
                aria-controls="login-password" @click="passwordVisible = !passwordVisible">
                <component :is="passwordVisible ? EyeOff : Eye" :size="18" aria-hidden="true" />
              </Button>
            </div>
            <FieldDescription v-if="registering" id="registration-password-rule">密码至少 12 个字符。</FieldDescription>
            <FieldDescription v-else-if="!operations">忘记密码？请联系运营核实租户归属后重置。</FieldDescription>
          </Field>
          <label v-if="registering && !operations" class="registration-consent">
            <input type="checkbox" required>
            <span>注册即表示您同意 <span class="policy-name">服务条款</span> 和 <span class="policy-name">隐私政策</span></span>
          </label>
          <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
          <Field>
            <Button type="submit" class="min-h-0 border-0" :disabled="submitting">
              {{ submitting ? '正在提交…' : registering ? '注册' : '登录' }}
            </Button>
            <FieldDescription v-if="!operations" class="text-center">
              {{ registering ? '已有账户？' : '没有账户？' }}
              <Button variant="link" type="button" class="h-auto min-h-0 border-0 bg-transparent p-0 hover:bg-transparent"
                :disabled="submitting" @click="emit('switchMode')">
                {{ registering ? '登录' : '注册' }}
              </Button>
            </FieldDescription>
          </Field>
        </FieldGroup>
      </form>
    </CardContent>
  </Card>
</template>

<style scoped>
.google-demo-button {
  min-height: 44px;
  gap: 12px;
  border: 1px solid #c3cfdb;
  border-radius: 6px;
  background: #fff;
  color: #17233d;
  box-shadow: none;
  font-weight: 400;
}
.google-demo-button:hover {
  border-color: #8faac5;
  background: #fff;
  color: #17233d;
}
.google-mark {
  font: 800 22px/1 Arial, sans-serif;
  color: #4285f4;
}
@supports (background-clip: text) {
  .google-mark {
    background: conic-gradient(from 75deg, #4285f4 0 32%, #34a853 32% 54%, #fbbc05 54% 69%, #ea4335 69%);
    background-clip: text;
    color: transparent;
  }
}
.registration-consent {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  color: #17233d;
  font-family: Arial, "Microsoft YaHei", "PingFang SC", sans-serif;
  font-size: 14px;
  font-weight: 400;
  line-height: 24px;
}
.registration-consent input[type="checkbox"] {
  flex: none;
  width: 16px;
  height: 16px;
  min-height: 0;
  margin: 4px 0 0;
  padding: 0;
  accent-color: #155ba7;
}
.policy-name { color: #086acb; }
</style>
