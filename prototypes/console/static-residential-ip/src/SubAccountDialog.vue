<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElSwitch, type FormInstance, type FormRules } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { socksUsername, subAccountNamePattern, subAccountPasswordPattern, subAccounts, type SubAccount } from './state'

export type SubAccountDialogMode = 'create' | 'edit' | 'password'

const props = defineProps<{ mode: SubAccountDialogMode, account?: SubAccount }>()
const emit = defineEmits<{ saved: [account: SubAccount] }>()
const open = defineModel<boolean>({ required: true })

const titles: Record<SubAccountDialogMode, string> = { create: '创建代理子账号', edit: '编辑子账号', password: '修改密码' }
const form = ref<FormInstance>()
const model = reactive({ name: '', password: '', remark: '', active: true, limited: false, limitGb: 10 })
const show = (field: string) => props.mode === 'create' || (props.mode === 'edit' ? ['remark', 'limit', 'active'] : ['password']).includes(field)
const minLimit = computed(() => props.mode === 'edit' && props.account ? Math.max(props.account.limitGb ?? 0, Math.ceil(props.account.usedGb)) : 1)

const rules: FormRules = {
  name: [
    { required: true, message: '请输入子账号名', trigger: 'blur' },
    { pattern: subAccountNamePattern, message: '4–8 位，以小写字母开头，只能包含小写字母和数字', trigger: 'blur' },
    { validator: (_r, value, cb) => subAccounts.value.some(a => a.name === value) ? cb(new Error('该子账号名已存在')) : cb(), trigger: 'blur' },
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { pattern: subAccountPasswordPattern, message: '6–12 位英文字母和数字，两类字符都要包含', trigger: 'blur' },
  ],
}

watch(open, (visible) => {
  if (!visible) return
  const a = props.account
  Object.assign(model, {
    name: a?.name ?? '',
    password: props.mode === 'password' ? '' : a?.password ?? '',
    remark: a?.remark ?? '',
    active: a?.active ?? true,
    limited: a?.limitGb !== undefined,
    limitGb: a?.limitGb ?? 10,
  })
  form.value?.clearValidate()
})

function generatePassword() {
  const letters = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ'
  const digits = '23456789'
  const pick = (chars: string) => chars[Math.floor(Math.random() * chars.length)]
  const body = Array.from({ length: 8 }, (_, i) => pick(i < 2 ? digits : letters + digits))
  model.password = body.sort(() => Math.random() - 0.5).join('')
  form.value?.validateField('password')
}

async function submit() {
  if (!await form.value?.validate().catch(() => false)) return
  const base = props.account
  emit('saved', {
    name: base?.name ?? model.name,
    password: model.password,
    remark: model.remark.trim(),
    active: model.active,
    usedGb: base?.usedGb ?? 0,
    limitGb: model.limited ? model.limitGb : undefined,
    createdAt: base?.createdAt ?? 0,
  })
  open.value = false
}
</script>

<template>
  <ElDialog v-model="open" :title="titles[mode]" width="min(520px, calc(100vw - 32px))" align-center>
    <ElForm ref="form" :model="model" :rules="rules" label-position="top" @submit.prevent="submit">
      <ElFormItem v-if="mode === 'create'" label="子账号名" prop="name">
        <ElInput v-model="model.name" maxlength="8" placeholder="例如 shop01" autocomplete="off" />
        <p class="form-hint">4–8 位，以小写字母开头，可包含小写字母和数字。创建后不能修改。</p>
      </ElFormItem>
      <ElFormItem v-else label="子账号名">
        <span class="mono-value">{{ account?.name }}</span>
      </ElFormItem>

      <ElFormItem v-if="show('password')" :label="mode === 'password' ? '新密码' : '密码'" prop="password">
        <div class="password-row">
          <ElInput v-model="model.password" maxlength="12" show-password autocomplete="new-password" />
          <ElButton :icon="Refresh" @click="generatePassword">随机生成</ElButton>
        </div>
        <p class="form-hint">6–12 位英文字母和数字，两类字符都要包含。</p>
      </ElFormItem>

      <ElFormItem v-if="show('remark')" label="备注">
        <ElInput v-model="model.remark" maxlength="40" show-word-limit placeholder="用途说明，仅自己可见" />
      </ElFormItem>

      <ElFormItem v-if="show('limit')" label="累计用量上限">
        <div class="limit-row">
          <ElSwitch v-model="model.limited" inline-prompt active-text="限制" inactive-text="不限" />
          <template v-if="model.limited">
            <ElInputNumber v-model="model.limitGb" :min="minLimit" :step="5" />
            <span class="form-unit">GB</span>
          </template>
        </div>
        <p class="form-hint">
          <template v-if="mode === 'edit' && account?.limitGb !== undefined">上限只能调高或取消，已用 {{ account.usedGb }} GB。</template>
          <template v-else>达到上限后拒绝新连接，已有连接照常计量。不设置则只受租户流量余额限制。</template>
        </p>
      </ElFormItem>

      <ElFormItem v-if="show('active')" label="状态">
        <ElSwitch v-model="model.active" active-text="启用" inactive-text="禁用" />
      </ElFormItem>

      <div v-if="mode !== 'edit'" class="socks-preview">
        <span>SOCKS5 用户名</span>
        <strong class="mono-value">{{ socksUsername(model.name || account?.name || '子账号名') }}</strong>
        <template v-if="mode === 'password'"><span class="form-hint">修改后新连接使用新密码，已有连接继续到正常结束。</span></template>
      </div>
    </ElForm>
    <template #footer>
      <ElButton @click="open = false">取消</ElButton>
      <ElButton type="primary" @click="submit">{{ mode === 'create' ? '创建' : '保存' }}</ElButton>
    </template>
  </ElDialog>
</template>
