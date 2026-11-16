<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ApiError } from './api/control'
import { getOverview, type Overview } from './api/overview'
import { claimStaticIPExpiryNotices } from './api/static-ip'
import { flowAmount, money } from './wallet-format'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ expired: []; loaded: [Overview] }>()
const controller = new AbortController()
const overview = ref<Overview>()
const failed = ref(false)
const claimedExpiry = ref(0)

async function load() {
  failed.value = false
  try {
    const result = await getOverview(props.token, controller.signal)
    if (controller.signal.aborted) return
    overview.value = result
    emit('loaded', result)
    if (result.notices.some(notice => notice.kind === 'static_ip_expiry')) {
      const claimed = await claimStaticIPExpiryNotices(props.token, controller.signal).catch(() => undefined)
      if (!controller.signal.aborted) claimedExpiry.value = claimed?.count ?? 0
    }
  } catch (error) {
    if (controller.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else failed.value = true
  }
}

onMounted(load)
onUnmounted(() => controller.abort())
</script>

<template>
  <div class="flex flex-col gap-4">
    <Card v-if="failed">
      <CardContent class="flex flex-wrap items-center gap-3">
        <p role="alert">暂时无法加载账户概览。</p>
        <Button type="button" variant="outline" @click="load">重新加载</Button>
      </CardContent>
    </Card>
    <p v-else-if="!overview" role="status">正在加载账户概览…</p>
    <template v-else>
      <div class="grid gap-4 sm:grid-cols-3">
        <Card>
          <CardHeader><h2 class="text-sm text-muted-foreground">钱包可用余额</h2></CardHeader>
          <CardContent class="break-all text-2xl font-semibold">{{ money(overview.wallet_minor) }}</CardContent>
        </Card>
        <Card>
          <CardHeader><h2 class="text-sm text-muted-foreground">动态流量余额</h2></CardHeader>
          <CardContent class="break-all text-2xl font-semibold">{{ flowAmount(overview.traffic_bytes) }}</CardContent>
        </Card>
        <Card>
          <CardHeader><h2 class="text-sm text-muted-foreground">可用静态 IP</h2></CardHeader>
          <CardContent class="text-2xl font-semibold">{{ overview.active_static_ip_count }} 个</CardContent>
        </Card>
      </div>
      <Card>
        <CardHeader><h2 class="font-semibold">快捷操作</h2></CardHeader>
        <CardContent class="flex flex-wrap items-center gap-3">
          <Button v-if="overview.products.some(p => p.kind === 'dynamic' && p.can_purchase)" as-child variant="outline">
            <RouterLink :to="{ path: '/', query: { view: 'packages' } }">购买流量</RouterLink>
          </Button>
          <Button v-if="overview.can_create_subaccount" as-child variant="outline">
            <RouterLink :to="{ path: '/', query: { view: 'subaccounts', create: '1' } }">创建子账号</RouterLink>
          </Button>
          <Button v-if="overview.products.some(p => p.kind === 'static' && p.can_purchase)" as-child variant="outline">
            <RouterLink :to="{ path: '/', query: { view: 'static' } }">购买静态 IP</RouterLink>
          </Button>
          <p v-if="!overview.can_create_subaccount" class="text-sm text-muted-foreground">暂时无法创建子账号。</p>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><h2 class="font-semibold">当前提醒</h2></CardHeader>
        <CardContent>
          <p v-if="overview.notices.length === 0" class="text-sm text-muted-foreground">暂无提醒。</p>
          <ul v-else class="flex list-none flex-col gap-2 p-0">
            <li v-for="notice in overview.notices" :key="notice.kind">
              <template v-if="notice.kind === 'low_traffic_balance'">
                动态流量余额已达到低余额提醒阈值。
                <RouterLink class="underline" :to="{ path: '/', query: { view: 'usage' } }">查看日用量</RouterLink>
              </template>
              <template v-else-if="notice.kind === 'static_ip_expiry'">
                <span role="status"><strong v-if="claimedExpiry">新提醒 · </strong></span>
                {{ notice.count }} 个静态 IP 将在 5 天内到期。
                <RouterLink class="underline" :to="{ path: '/', query: { view: 'ips' } }">查看我的 IP</RouterLink>
              </template>
            </li>
          </ul>
        </CardContent>
      </Card>
    </template>
  </div>
</template>
