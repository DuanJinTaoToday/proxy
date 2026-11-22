<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElCheckbox, ElForm, ElFormItem, ElInput, ElMessage,
  ElMessageBox, ElSwitch, ElTable, ElTableColumn, ElTag, ElTooltip,
} from 'element-plus'
import OpsDialog from '@/shared/ui/OpsDialog.vue'
import { ApiError } from '@/api/client'
import { getOpsCatalog, setProductSale, updateTrafficPackage, type OpsTrafficPackage, type ProductKind } from '@/api/products'
import { hasOperatorPermission } from '@/features/auth'
import { formatYuan } from '@/shared/lib/money'
import { bytesPerGB, formatGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

type Catalog = Awaited<ReturnType<typeof getOpsCatalog>>
const saleProducts: ProductKind[] = ['dynamic', 'static']
const { t, te, locale } = useI18n()
const genericError = useErrorMessage()
const canBill = computed(() => hasOperatorPermission('manage_billing'))
const canRead = computed(() => canBill.value || hasOperatorPermission('read_operations'))
const opsCatalog = shallowRef<Catalog>()
const loadError = shallowRef<unknown>()
const mutationError = shallowRef<unknown>()
const busy = ref(false)
const loading = ref(false)
const locked = computed(() => !canBill.value || busy.value || loading.value || Boolean(loadError.value))
const editing = shallowRef<OpsTrafficPackage>()
const price = ref('')
const opening = ref<ProductKind>()
const acceptanceEvidence = ref('')
const quoteReady = ref(false)
const controller = new AbortController()
onScopeDispose(() => controller.abort())

const money = (minor: bigint) => formatYuan(minor, locale.value)
const priceLabel = (minor: bigint) => minor > 0n ? money(minor) : t('catalog.unpriced')
const capacityLabel = (bytes: bigint) => formatGB(bytes, locale.value)
const listingLabel = (listed: boolean) => t(listed ? 'catalog.listed' : 'catalog.unlisted')
const saleLabel = (open: boolean) => t(open ? 'catalog.open' : 'catalog.saleDisabled')
const productName = (kind: string) => t('catalog.' + kind)
const perGB = (priceMinor: bigint, bytes: bigint) => (priceMinor * bytesPerGB + bytes / 2n) / bytes

const dateTime = computed(() => new Intl.DateTimeFormat(locale.value, {
  timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
}))
const headerCellStyle = {
  height: 'var(--catalog-table-header-height)', color: 'var(--color-text-secondary)',
  fontSize: 'var(--font-size-xs)', fontWeight: 'var(--font-weight-semibold)',
}
const cellStyle = { height: 'var(--catalog-table-row-height)' }

function packageComparison(priceMinor: bigint, capacityBytes: bigint) {
  const baseline = opsCatalog.value?.packages.find(pkg => pkg.id === '5gb')
  if (!baseline?.priceMinor || !priceMinor) return '—'
  const base = baseline.priceMinor * capacityBytes
  const difference = base - priceMinor * baseline.capacityBytes
  const percent = ((difference < 0n ? -difference : difference) * 100n + base / 2n) / base
  if (!percent) return t('catalog.baseline')
  return t(difference < 0n ? 'catalog.higher' : 'catalog.lower', { percent: percent.toString() })
}

const audit = computed(() => (opsCatalog.value?.changes ?? []).flatMap(change => {
  const { before, after } = change
  const when = dateTime.value.format(new Date(change.at))
  const row = (field: string, target: string, oldValue: string, newValue: string) => ({
    id: change.id + '-' + field, at: change.at, when, operator: change.operator, target, before: oldValue, after: newValue,
  })
  if (change.kind === 'sale') {
    return [row('sale', t('catalog.auditSale', { name: productName(change.target) }), saleLabel(before.open), saleLabel(after.open))]
  }
  const target = (field: string) => t('catalog.auditPackage', {
    capacity: capacityLabel(after.capacityBytes), field: t('catalog.' + field),
  })
  return [
    ...(before.priceMinor !== after.priceMinor ? [row('price', target('price'), priceLabel(before.priceMinor), priceLabel(after.priceMinor))] : []),
    ...(before.listed !== after.listed ? [row('status', target('status'), listingLabel(before.listed), listingLabel(after.listed))] : []),
    ...(before.capacityBytes !== after.capacityBytes ? [row('capacity', target('capacity'), capacityLabel(before.capacityBytes) + ' GB', capacityLabel(after.capacityBytes) + ' GB')] : []),
  ]
}))

function errorMessage(error: unknown) {
  const key = error instanceof ApiError ? 'catalog.errors.' + error.reason : ''
  return key && te(key) ? t(key) : genericError(error)
}

async function load() {
  loadError.value = undefined
  loading.value = true
  try {
    opsCatalog.value = await getOpsCatalog(controller.signal)
  } catch (error) {
    if (!controller.signal.aborted) loadError.value = error
  } finally {
    loading.value = false
  }
}

async function save(write: () => Promise<unknown>, complete: () => void) {
  if (locked.value) return
  mutationError.value = undefined
  busy.value = true
  try {
    await write()
    if (!controller.signal.aborted) {
      complete()
      await load()
    }
  } catch (error) {
    if (!controller.signal.aborted) {
      mutationError.value = error
      if (error instanceof ApiError && error.reason === 'PACKAGE_CHANGED') {
        editing.value = undefined
        await load()
      }
    }
  } finally {
    busy.value = false
  }
}

async function toggleSale(kind: ProductKind) {
  if (locked.value) return
  mutationError.value = undefined
  if (!opsCatalog.value?.sales[kind]) {
    opening.value = kind
    acceptanceEvidence.value = ''
    quoteReady.value = false
    return
  }
  const name = productName(kind)
  const confirmed = await ElMessageBox.confirm(t('catalog.closeHint'), t('catalog.closeTitle', { name }), {
    confirmButtonText: t('catalog.close'), cancelButtonText: t('catalog.cancel'), type: 'warning', autofocus: false,
  }).then(() => true, () => false)
  if (confirmed) {
    await save(() => setProductSale(kind, { open: false }, controller.signal), () => ElMessage.success(t('catalog.saleClosed', { name })))
  }
}

function openSale() {
  const kind = opening.value
  if (!kind || !quoteReady.value || !acceptanceEvidence.value.trim()) return
  return save(
    () => setProductSale(kind, { open: true, evidence: acceptanceEvidence.value.trim() }, controller.signal),
    () => {
      opening.value = undefined
      ElMessage.success(t('catalog.saleOpened', { name: productName(kind) }))
    },
  )
}

async function toggleListed(id: string) {
  const tier = opsCatalog.value?.packages.find(pkg => pkg.id === id)
  if (!tier) return
  if (locked.value || (!tier.listed && !tier.priceMinor)) return
  const listed = !tier.listed
  const purchasable = opsCatalog.value?.packages.filter(pkg => pkg.listed && pkg.priceMinor > 0n)
  if (!listed && tier.priceMinor > 0n && purchasable?.length === 1) {
    const confirmed = await ElMessageBox.confirm(t('catalog.lastPackageHint'), t('catalog.lastPackageTitle'), {
      confirmButtonText: t('catalog.unlistAnyway'), cancelButtonText: t('catalog.cancel'), type: 'warning',
    }).then(() => true, () => false)
    if (!confirmed) return
  }
  await save(
    () => updateTrafficPackage(tier, { priceMinor: tier.priceMinor, listed }, controller.signal),
    () => ElMessage.success(t(listed ? 'catalog.packageListed' : 'catalog.packageUnlisted', { capacity: capacityLabel(tier.capacityBytes) })),
  )
}

function edit(id: string) {
  const tier = opsCatalog.value?.packages.find(pkg => pkg.id === id)
  if (!tier) return
  editing.value = tier
  mutationError.value = undefined
  price.value = tier.priceMinor ? (tier.priceMinor / 100n) + '.' + (tier.priceMinor % 100n).toString().padStart(2, '0') : ''
}

const newPrice = computed(() => {
  const parts = /^(\d{1,17})(?:\.(\d{1,2}))?$/.exec(price.value.trim())
  if (!parts) return
  const minor = BigInt(parts[1]) * 100n + BigInt((parts[2] ?? '').padEnd(2, '0'))
  if (minor > 0n && minor <= 9_223_372_036_854_775_807n) return minor
})

function savePrice() {
  const tier = editing.value
  const priceMinor = newPrice.value
  if (!tier || !priceMinor || locked.value) return
  if (priceMinor === tier.priceMinor) {
    editing.value = undefined
    return
  }
  return save(
    () => updateTrafficPackage(tier, { priceMinor, listed: tier.listed }, controller.signal),
    () => {
      editing.value = undefined
      ElMessage.success(t('catalog.priceUpdated'))
    },
  )
}

if (canRead.value) void load()
</script>

<template>
  <h1 class="ops-page-title">{{ t('nav.opsProducts') }}</h1>
  <ElAlert v-if="!canRead" type="warning" :title="t('errors.ACCESS_DENIED')" :closable="false" show-icon />
  <template v-else>
    <ElAlert v-if="!canBill" type="info" :closable="false" show-icon class="page-alert" :title="t('catalog.readOnly')" />
    <ElAlert v-if="loadError" type="error" :closable="false" show-icon class="page-alert" :title="errorMessage(loadError)">
      <ElButton @click="load">{{ t('catalog.retry') }}</ElButton>
    </ElAlert>
    <ElAlert v-if="mutationError && !editing && !opening" type="error" :closable="false" show-icon class="page-alert" :title="errorMessage(mutationError)" />
    <p v-if="loading && !opsCatalog" role="status">{{ t('catalog.loading') }}</p>

    <template v-if="opsCatalog">
      <div class="sale-grid workspace-two-columns">
        <section v-for="kind in saleProducts" :key="kind" class="catalog-card sale-card">
          <div>
            <h2>{{ productName(kind) }}</h2>
            <p class="card-desc">{{ t('catalog.' + kind + 'Hint') }}</p>
          </div>
          <div class="sale-switch">
            <ElTag :type="opsCatalog.sales[kind] ? 'success' : 'info'" disable-transitions>{{ t(opsCatalog.sales[kind] ? 'catalog.onSale' : 'catalog.closed') }}</ElTag>
            <ElTooltip :disabled="canBill" :content="t('catalog.billingOnly')" placement="top">
              <span @keydown.enter.stop.prevent>
                <ElSwitch :model-value="opsCatalog.sales[kind]" :disabled="locked" :aria-label="t('catalog.switchLabel', { name: productName(kind) })" @change="toggleSale(kind)" />
              </span>
            </ElTooltip>
          </div>
        </section>
      </div>

      <section class="catalog-card table-card" aria-labelledby="package-title">
        <div class="table-toolbar">
          <h2 id="package-title">{{ t('catalog.packageTitle') }}</h2>
          <p class="card-desc">{{ t('catalog.packageHint') }}</p>
        </div>
        <ElTable :data="opsCatalog.packages" row-key="id" class="catalog-table" :empty-text="t('catalog.noPackages')"
          :header-cell-style="headerCellStyle" :cell-style="cellStyle">
          <ElTableColumn :label="t('catalog.capacity')" min-width="100">
            <template #default="{ row }"><strong>{{ capacityLabel(row.capacityBytes) }} GB</strong></template>
          </ElTableColumn>
          <ElTableColumn :label="t('catalog.price')" min-width="110" align="right">
            <template #default="{ row }"><span class="tabular price">{{ priceLabel(row.priceMinor) }}</span></template>
          </ElTableColumn>
          <ElTableColumn :label="t('catalog.perGb')" min-width="110" align="right">
            <template #default="{ row }"><span class="tabular price">{{ row.priceMinor ? money(perGB(row.priceMinor, row.capacityBytes)) : '—' }}</span></template>
          </ElTableColumn>
          <ElTableColumn :label="t('catalog.compare')" min-width="120" align="right">
            <template #default="{ row }">{{ packageComparison(row.priceMinor, row.capacityBytes) }}</template>
          </ElTableColumn>
          <ElTableColumn :label="t('catalog.status')" min-width="90">
            <template #default="{ row }"><ElTag :type="row.listed ? 'success' : 'info'" size="small" disable-transitions>{{ listingLabel(row.listed) }}</ElTag></template>
          </ElTableColumn>
          <ElTableColumn :width="locale === 'en' ? 164 : 140" align="right">
            <template #default="{ row }">
              <ElTooltip :disabled="canBill && (row.listed || Boolean(row.priceMinor))" :content="t(canBill ? 'catalog.priceRequired' : 'catalog.billingOnly')" placement="left">
                <span class="row-actions">
                  <ElButton link type="primary" :disabled="locked" @click="edit(row.id)">{{ t('catalog.editPrice') }}</ElButton>
                  <ElButton link :type="row.listed ? 'danger' : 'primary'" :disabled="locked || (!row.listed && !row.priceMinor)" @click="toggleListed(row.id)">{{ t(row.listed ? 'catalog.unlist' : 'catalog.list') }}</ElButton>
                </span>
              </ElTooltip>
            </template>
          </ElTableColumn>
        </ElTable>
      </section>

      <section class="catalog-card table-card" aria-labelledby="audit-title">
        <div class="table-toolbar"><h2 id="audit-title">{{ t('catalog.auditTitle') }}</h2></div>
        <ElTable :data="audit" row-key="id" class="catalog-table" :empty-text="t('catalog.noChanges')"
          :header-cell-style="headerCellStyle" :cell-style="cellStyle">
          <ElTableColumn :label="t('catalog.auditTime')" min-width="150">
            <template #default="{ row }"><time :datetime="row.at">{{ row.when }}</time></template>
          </ElTableColumn>
          <ElTableColumn prop="target" :label="t('catalog.auditTarget')" min-width="200" />
          <ElTableColumn :label="t('catalog.auditValues')" min-width="200">
            <template #default="{ row }"><span class="audit-before">{{ row.before }}</span> → <strong>{{ row.after }}</strong></template>
          </ElTableColumn>
          <ElTableColumn prop="operator" :label="t('catalog.auditOperator')" width="100" show-overflow-tooltip />
        </ElTable>
      </section>
    </template>

    <OpsDialog class="catalog-price-dialog" :model-value="Boolean(editing)" :title="editing && t('catalog.editTitle', { capacity: capacityLabel(editing.capacityBytes) })"
      width="min(var(--catalog-price-dialog-width), calc(100vw - var(--space-6)))" append-to-body
      :show-close="!busy" :close-on-click-modal="!busy" :close-on-press-escape="!busy" @update:model-value="editing = undefined">
      <ElForm v-if="editing" label-position="top" @submit.prevent="savePrice">
        <ElFormItem :label="t('catalog.yuanPrice')" :error="price && !newPrice ? t('catalog.invalidPrice') : undefined">
          <ElInput v-model="price" inputmode="decimal" :disabled="busy" maxlength="20" :input-style="{ textAlign: 'center' }" />
        </ElFormItem>
        <dl class="summary-total">
          <div><dt>{{ t('catalog.currentPrice') }}</dt><dd>{{ priceLabel(editing.priceMinor) }}</dd></div>
          <div><dt>{{ t('catalog.newPerGb') }}</dt><dd>{{ newPrice ? money(perGB(newPrice, editing.capacityBytes)) : '—' }}</dd></div>
        </dl>
        <ElAlert v-if="mutationError" type="error" :closable="false" show-icon :title="errorMessage(mutationError)" />
      </ElForm>
      <template #footer>
        <ElButton class="ops-dialog-action" :disabled="busy" @click="editing = undefined">{{ t('catalog.cancel') }}</ElButton>
        <ElButton class="ops-dialog-action" type="primary" :loading="busy" :disabled="locked || !newPrice" @click="savePrice">{{ t('catalog.save') }}</ElButton>
      </template>
    </OpsDialog>

    <OpsDialog :model-value="Boolean(opening)" :title="opening && t('catalog.openTitle', { name: productName(opening) })"
      width="min(var(--catalog-price-dialog-width), calc(100vw - var(--space-6)))" append-to-body
      :show-close="!busy" :close-on-click-modal="!busy" :close-on-press-escape="!busy" @update:model-value="opening = undefined">
      <p v-if="opening" class="sale-hint">{{ t('catalog.openHint', { name: productName(opening) }) }}</p>
      <ElForm label-position="top" @submit.prevent="openSale">
        <ElFormItem :label="t('catalog.acceptanceEvidence')">
          <ElInput v-model="acceptanceEvidence" :disabled="busy" maxlength="1000" type="textarea" />
        </ElFormItem>
        <ElCheckbox v-model="quoteReady" :disabled="busy">{{ t('catalog.quoteReady') }}</ElCheckbox>
        <ElAlert v-if="mutationError" type="error" :closable="false" show-icon :title="errorMessage(mutationError)" />
      </ElForm>
      <template #footer>
        <ElButton class="ops-dialog-action" :disabled="busy" @click="opening = undefined">{{ t('catalog.cancel') }}</ElButton>
        <ElButton class="ops-dialog-action" type="primary" :loading="busy" :disabled="locked || !quoteReady || !acceptanceEvidence.trim()" @click="openSale">{{ t('catalog.open') }}</ElButton>
      </template>
    </OpsDialog>
  </template>
</template>

<style>
.ops-dialog.catalog-price-dialog { line-height: normal; }
</style>

<style scoped>
.catalog-card {
  min-width: 0;
  overflow: hidden;
  background: var(--color-surface);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--ops-card-radius);
}
.sale-grid {
  gap: var(--space-4);
  margin-block-end: var(--space-4);
}
.sale-card {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-5);
}
.sale-card > div:first-child { flex: 1 1 var(--catalog-sale-text-width); }
.sale-card h2, .table-toolbar h2 { font-size: var(--font-size-lg); line-height: var(--space-5); }
.card-desc {
  margin-block-start: var(--ops-detail-meta-gap);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: normal;
}
.sale-card .card-desc { margin-block-start: var(--catalog-sale-description-gap); }
.sale-switch { display: flex; align-items: center; gap: var(--space-3); flex: none; }
.page-alert, .sale-hint { margin-block-end: var(--space-4); }
.table-card + .table-card { margin-block-start: var(--space-4); }
.table-toolbar {
  padding: var(--space-3) var(--space-4);
  border-block-end: var(--border-width) solid var(--color-border);
}
.catalog-table {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  --el-table-current-row-bg-color: var(--color-surface-muted);
  font-size: var(--font-size-sm);
}
.price { color: var(--color-text); font-weight: var(--font-weight-semibold); }
.row-actions { display: inline-flex; flex-wrap: wrap; justify-content: end; gap: var(--space-3); }
.row-actions .el-button { margin-inline-start: 0; font-size: var(--font-size-sm); }
.audit-before { color: var(--color-text-secondary); text-decoration: line-through; }
.summary-total {
  display: grid;
  gap: var(--space-2);
  margin: var(--space-4) 0 0;
  padding-block-start: var(--space-3);
  border-block-start: var(--border-width) solid var(--color-border);
  font-size: var(--font-size-sm);
}
.summary-total div { display: flex; justify-content: space-between; gap: var(--space-2); }
.summary-total dt { color: var(--color-text-secondary); }
.summary-total dd { margin: 0; font-weight: var(--font-weight-semibold); font-variant-numeric: tabular-nums; }
.summary-total + .el-alert { margin-block-start: var(--space-3); }
</style>
