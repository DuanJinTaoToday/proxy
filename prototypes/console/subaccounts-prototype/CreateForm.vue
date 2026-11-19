<script setup lang="ts">
import { nextTick, ref, useId } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { ApiError } from '@console/api/control'
import { createSubaccount } from '@console/api/subaccounts'
import type { Subaccount } from '@console/api/subaccounts'
import { Button } from '@console/ui/button'
import FieldLabel from '@console/ui/field/FieldLabel.vue'
import Input from '@console/ui/input/Input.vue'

const emit = defineEmits<{ created: [Subaccount]; cancel: [] }>()
const id = useId()
const name = ref('')
const password = ref('')
const remark = ref('')
const limit = ref('')
const active = ref(true)
const busy = ref(false)
const error = ref('')
const duplicate = ref(false)
const form = ref<HTMLFormElement>()

async function save() {
  if (busy.value) return
  busy.value = true
  error.value = ''
  duplicate.value = false
  try {
    const [whole = '', fraction = ''] = limit.value.split('.')
    const limit_bytes = whole ? String(BigInt(whole) * 1_000_000_000n + BigInt(fraction.padEnd(9, '0'))) : undefined
    const account = await createSubaccount('prototype', {
      name: name.value, password: password.value, active: active.value, remark: remark.value, limit_bytes,
    })
    emit('created', account)
  } catch (cause) {
    duplicate.value = cause instanceof ApiError && cause.reason === 'SUBACCOUNT_NAME_EXISTS'
    error.value = duplicate.value ? '名称已存在，请换一个名称。' : '创建失败，请稍后重试。'
    await nextTick()
    if (duplicate.value) form.value?.querySelector('input')?.focus()
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form ref="form" class="prototype-form" @submit.prevent="save">
    <div class="prototype-field">
      <FieldLabel :for="id + '-name'">子账号名称</FieldLabel>
      <Input :id="id + '-name'" v-model="name" required pattern="[a-z][a-z0-9]{3,7}" maxlength="8"
        autocomplete="off" :aria-invalid="duplicate || undefined" :aria-describedby="id + (duplicate ? '-error' : '-name-help')"
        @update:model-value="duplicate = false; error = ''" />
      <p :id="id + '-name-help'" class="prototype-help">4–8 位，以小写字母开头，可包含小写字母和数字。</p>
      <p v-if="duplicate" :id="id + '-error'" class="prototype-error" role="alert">{{ error }}</p>
    </div>
    <div class="prototype-field">
      <FieldLabel :for="id + '-password'">密码</FieldLabel>
      <Input :id="id + '-password'" v-model="password" type="password" required autocomplete="new-password"
        pattern="(?=.*[A-Za-z])(?=.*[0-9])[A-Za-z0-9]{6,12}" :aria-describedby="id + '-password-help'" />
      <p :id="id + '-password-help'" class="prototype-help">6–12 位，须包含英文字母和数字。</p>
    </div>
    <details class="prototype-options">
      <summary>更多设置 <ChevronDown :size="16" aria-hidden="true" /></summary>
      <div class="prototype-fields">
        <div class="prototype-field">
          <FieldLabel :for="id + '-remark'">备注</FieldLabel>
          <Input :id="id + '-remark'" v-model="remark" maxlength="512" />
        </div>
        <div class="prototype-field">
          <FieldLabel :for="id + '-limit'">累计上限（GB，留空不限）</FieldLabel>
          <Input :id="id + '-limit'" v-model="limit" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,9})?" />
        </div>
        <label class="prototype-checkbox"><input v-model="active" type="checkbox">启用</label>
      </div>
    </details>
    <p v-if="error && !duplicate" class="prototype-error" role="alert">{{ error }}</p>
    <div class="prototype-actions">
      <Button type="button" variant="outline" :disabled="busy" @click="emit('cancel')">取消</Button>
      <Button type="submit" class="prototype-primary" :disabled="busy">{{ busy ? '正在创建…' : '创建子账号' }}</Button>
    </div>
  </form>
</template>
