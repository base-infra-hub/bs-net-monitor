<script setup lang="ts">
import { ref, onMounted, h, computed, inject, watch, type Ref } from 'vue'
import {
  NButton,
  NDataTable,
  NModal,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSwitch,
  NSelect,
  NSpace,
  NPopconfirm,
  NTag,
  NCard,
  NEmpty,
  NUpload,
  NIcon,
  NAlert,
  useMessage,
  type DataTableColumns,
  type UploadFileInfo,
  type FormRules,
} from 'naive-ui'
import {
  Add,
  CloudUploadOutline,
  CloudDownloadOutline,
  DownloadOutline,
  TrashOutline,
  CheckmarkCircleOutline,
  CloseCircleOutline,
} from '@vicons/ionicons5'
import { ipApi, tacticsApi, type IP, type Tactics } from '../api'
import { fmtDateTime } from '../utils/datetime'

const message = useMessage()

const tacticsList = ref<Tactics[]>([])
const ipList = ref<IP[]>([])
const loading = ref(false)
const filterTacticsId = ref<number | null>(null)
const filterEnabled = ref<string | null>(null)
const current = ref(1)
const pageSize = ref(15)
const total = ref(0)
const selectedIps = ref<number[]>([])

const currentTenant = inject<Ref<string>>('currentTenant')

const ipModalVisible = ref(false)
const ipModalTitle = ref('新增 IP')
const ipForm = ref<Partial<IP>>({
  name: '',
  ip: '',
  position: '',
  remark: '',
  tacticsId: undefined,
  enabled: true,
})
const editingIpId = ref<number | null>(null)

const tacticsModalVisible = ref(false)
const tacticsModalTitle = ref('新增策略')
const tacticsForm = ref<Partial<Tactics>>({
  name: '',
  intervalMs: 60000,
  timeoutMs: 3000,
  unstableMs: 0,
  enabled: true,
})
const editingTacticsId = ref<number | null>(null)
const tacticsFormRef = ref<any>(null)

const tacticsRules: FormRules = {
  name: { required: true, message: '请输入策略名称', trigger: 'blur' },
  intervalMs: { required: true, type: 'number', message: '请输入检测间隔', trigger: 'blur' },
  timeoutMs: { required: true, type: 'number', message: '请输入超时时间', trigger: 'blur' },
}

const importModalVisible = ref(false)
const importTacticsId = ref<number | null>(null)
const importFileList = ref<UploadFileInfo[]>([])

const loadData = async () => {
  loading.value = true
  try {
    const [ipRes, tacticsRes] = await Promise.all([
      ipApi.list({
        current: current.value,
        size: pageSize.value,
        tacticsId: filterTacticsId.value,
        enabled: enabledValue(),
      }),
      tacticsApi.list(),
    ])
    ipList.value = ipRes.data?.records || []
    total.value = ipRes.data?.total || 0
    tacticsList.value = tacticsRes.data || []
  } finally {
    loading.value = false
  }
}

// 监听租户切换，刷新数据，不再进行强制整页销毁重建
watch(
  () => currentTenant?.value,
  () => {
    current.value = 1
    selectedIps.value = []
    loadData()
  }
)

onMounted(loadData)

const filterOptions = computed(() => [
  { label: '全部策略', value: null as any },
  ...tacticsList.value.map((t) => ({
    label: `${t.name} (${t.tacticsId})`,
    value: t.tacticsId,
  })),
])

const enabledOptions = [
  { label: '全部状态', value: null as any },
  { label: '启用', value: 'true' },
  { label: '停用', value: 'false' },
]

const enabledValue = () => {
  if (filterEnabled.value === null) return null
  return filterEnabled.value === 'true'
}

const onSearch = () => {
  current.value = 1
  loadData()
}

const onPageChange = (page: number) => {
  current.value = page
  loadData()
}

const onPageSizeChange = (size: number) => {
  pageSize.value = size
  current.value = 1
  loadData()
}

const openIpModal = (row?: IP) => {
  if (row) {
    ipModalTitle.value = '编辑 IP 设备'
    editingIpId.value = row.ipId
    ipForm.value = {
      name: row.name,
      ip: row.ip,
      position: row.position,
      remark: row.remark,
      tacticsId: row.tacticsId,
      enabled: row.enabled,
    }
  } else {
    ipModalTitle.value = '新增 IP 设备'
    editingIpId.value = null
    ipForm.value = {
      name: '',
      ip: '',
      position: '',
      remark: '',
      tacticsId: filterTacticsId.value || undefined,
      enabled: true,
    }
  }
  ipModalVisible.value = true
}

const saveIp = async () => {
  try {
    if (editingIpId.value) {
      await ipApi.update(editingIpId.value, ipForm.value)
      message.success('更新成功')
    } else {
      await ipApi.create(ipForm.value)
      message.success('创建成功')
    }
    ipModalVisible.value = false
    loadData()
  } catch {}
}

const deleteIp = async (ipId: number) => {
  try {
    await ipApi.delete(ipId)
    message.success('删除成功')
    loadData()
  } catch {}
}

const batchUpdateEnabled = async (enabled: boolean) => {
  if (selectedIps.value.length === 0) {
    message.warning('请先选择要操作的 IP 设备')
    return
  }
  try {
    await ipApi.batchUpdateEnabled({ ipIds: selectedIps.value, enabled })
    message.success('批量更新成功')
    selectedIps.value = []
    loadData()
  } catch {}
}

const batchDelete = async () => {
  if (selectedIps.value.length === 0) {
    message.warning('请先选择要操作的 IP 设备')
    return
  }
  try {
    await ipApi.batchDelete({ ipIds: selectedIps.value })
    message.success('批量删除成功')
    selectedIps.value = []
    loadData()
  } catch {}
}

const openImportModal = () => {
  importTacticsId.value = filterTacticsId.value
  importFileList.value = []
  importModalVisible.value = true
}

const handleImport = async () => {
  if (!importTacticsId.value) {
    message.warning('请选择要导入的策略组')
    return
  }
  const file = importFileList.value[0]?.file
  if (!file) {
    message.warning('请选择要导入的 Excel 文件')
    return
  }

  const formData = new FormData()
  formData.append('file', file)
  formData.append('tacticsId', String(importTacticsId.value))

  try {
    const res = await ipApi.import(formData)
    message.success(`成功导入 ${res.data.imported} 个 IP`)
    importModalVisible.value = false
    importFileList.value = []
    loadData()
  } catch {}
}

const handleExport = async () => {
  try {
    const blob = await ipApi.export(filterTacticsId.value)
    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    let filename = 'ips_all.xlsx'
    if (filterTacticsId.value) {
      const tactics = tacticsList.value.find((t) => t.tacticsId === filterTacticsId.value)
      filename = tactics ? `ips_${tactics.name}.xlsx` : `ips_tactics_${filterTacticsId.value}.xlsx`
    }
    link.download = filename
    link.click()
    window.URL.revokeObjectURL(link.href)
    message.success('导出成功')
  } catch {}
}

const downloadTemplate = async () => {
  try {
    const blob = await ipApi.template()
    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    link.download = 'ip_import_template.xlsx'
    link.click()
    window.URL.revokeObjectURL(link.href)
  } catch {}
}

const openTacticsModal = (row?: Tactics) => {
  if (row) {
    tacticsModalTitle.value = '编辑检测策略'
    editingTacticsId.value = row.tacticsId
    tacticsForm.value = {
      name: row.name,
      intervalMs: row.intervalMs,
      timeoutMs: row.timeoutMs,
      unstableMs: row.unstableMs,
      enabled: row.enabled,
    }
  } else {
    tacticsModalTitle.value = '新增检测策略'
    editingTacticsId.value = null
    tacticsForm.value = {
      name: '',
      intervalMs: 60000,
      timeoutMs: 3000,
      unstableMs: 0,
      enabled: true,
    }
  }
  tacticsModalVisible.value = true
}

const saveTactics = async () => {
  try {
    await tacticsFormRef.value?.validate()
  } catch {
    return
  }

  const timeout = tacticsForm.value.timeoutMs || 0
  const unstable = tacticsForm.value.unstableMs || 0
  if (unstable > timeout) {
    message.warning('不稳定阈值不能大于超时时间')
    return
  }

  try {
    if (editingTacticsId.value) {
      await tacticsApi.update(editingTacticsId.value, tacticsForm.value)
      message.success('更新成功')
    } else {
      await tacticsApi.create(tacticsForm.value)
      message.success('创建成功')
    }
    tacticsModalVisible.value = false
    loadData()
  } catch {}
}

const deleteTactics = async (tacticsId: number) => {
  try {
    await tacticsApi.delete(tacticsId)
    message.success('删除成功')
    if (filterTacticsId.value === tacticsId) {
      filterTacticsId.value = null
    }
    loadData()
  } catch {}
}

const tacticsOptions = computed(() =>
  tacticsList.value.map((t) => ({
    label: `${t.name} (${t.intervalMs}ms)`,
    value: t.tacticsId,
  }))
)

const currentTacticsName = computed(() => {
  if (!filterTacticsId.value) return ''
  const t = tacticsList.value.find((x) => x.tacticsId === filterTacticsId.value)
  return t ? t.name : String(filterTacticsId.value)
})

const ipColumns: DataTableColumns<IP> = [
  { type: 'selection', align: 'center' },
  { title: 'ID', key: 'ipId', width: 60, align: 'center' },
  { title: '设备名称', key: 'name', ellipsis: { tooltip: true }, align: 'center' },
  { title: 'IP 地址', key: 'ip', width: 140, align: 'center', render: (row) => h('code', { class: 'ip-code' }, row.ip) },
  { title: '物理位置', key: 'position', ellipsis: { tooltip: true }, align: 'center' },
  { title: '备注信息', key: 'remark', ellipsis: { tooltip: true }, align: 'center' },
  {
    title: '关联策略组',
    key: 'tacticsName',
    width: 140,
    ellipsis: { tooltip: true },
    align: 'center',
    render: (row) => h(NTag, { type: 'info', size: 'small', round: true }, { default: () => row.tacticsName || '未关联' })
  },
  {
    title: '使用状态',
    key: 'enabled',
    width: 90,
    align: 'center',
    render: (row) =>
      h(
        NTag,
        { type: row.enabled ? 'success' : 'default', size: 'small', class: 'status-tag' },
        { default: () => (row.enabled ? '已启用' : '已停用') }
      ),
  },
  {
    title: '录入时间',
    key: 'createdAt',
    width: 170,
    align: 'center',
    render: (row) => {
      if (!row.createdAt) return '--'
      return fmtDateTime(row.createdAt)
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 120,
    fixed: 'right',
    align: 'center',
    render: (row) =>
      h(NSpace, { size: 'small' }, {
        default: () => [
          h(
            NButton,
            { size: 'small', text: true, type: 'primary', class: 'btn-action', onClick: () => openIpModal(row) },
            { default: () => '编辑' }
          ),
          h(
            NPopconfirm,
            { onPositiveClick: () => deleteIp(row.ipId) },
            {
              trigger: () =>
                h(NButton, { size: 'small', text: true, type: 'error', class: 'btn-action' }, { default: () => '删除' }),
              default: () => '确定删除该 IP 吗？此操作不可撤销。',
            }
          ),
        ],
      }),
  },
]
</script>

<template>
  <div class="manage-container">
    <h2 class="page-title">地址监控管理</h2>

    <!-- 策略组横向看板 -->
    <div class="glass-card tactics-section">
      <div class="section-header">
        <div class="header-left">
          <span class="section-title">策略配置组</span>
          <span class="section-subtitle">配置 IP 探测的周期频率和超时阈值</span>
        </div>
        <n-button type="primary" size="small" class="glow-button" @click="openTacticsModal()">
          <template #icon>
            <n-icon><Add /></n-icon>
          </template>
          新增策略
        </n-button>
      </div>

      <div class="tactics-scroll">
        <div
          v-for="t in tacticsList"
          :key="t.tacticsId"
          class="t-card"
          :class="{ active: filterTacticsId === t.tacticsId }"
          @click="filterTacticsId = filterTacticsId === t.tacticsId ? null : t.tacticsId; onSearch()"
        >
          <div class="card-glow-effect"></div>
          <div class="t-card-header">
            <span class="t-name">{{ t.name }}</span>
            <n-tag
              :type="t.enabled ? 'success' : 'default'"
              size="small"
              round
            >
              {{ t.enabled ? '激活' : '停用' }}
            </n-tag>
          </div>
          <div class="t-card-body">
            <div class="meta-item">
              <span class="label">检测周期:</span>
              <span class="val">{{ t.intervalMs }}ms</span>
            </div>
            <div class="meta-item">
              <span class="label">超时阈值:</span>
              <span class="val">{{ t.timeoutMs }}ms</span>
            </div>
            <div class="meta-item">
              <span class="label">不稳定判定:</span>
              <span class="val">{{ t.unstableMs }}ms</span>
            </div>
          </div>
          <div class="t-card-footer" @click.stop>
            <n-space size="medium">
              <span class="text-btn edit" @click="openTacticsModal(t)">编辑</span>
              <n-popconfirm @positive-click="deleteTactics(t.tacticsId)">
                <template #trigger>
                  <span class="text-btn delete">删除</span>
                </template>
                确定删除该策略组吗？其下的 IP 将失去策略。
              </n-popconfirm>
            </n-space>
          </div>
        </div>

        <div v-if="tacticsList.length === 0" class="empty-tactics">
          <n-empty description="暂无策略组配置" />
        </div>
      </div>
    </div>

    <!-- IP 列表面板 -->
    <div class="glass-card ip-section">
      <div class="section-header list-header">
        <div class="header-left inline-filters">
          <span class="section-title">IP 监控列表</span>
          <n-select
            v-model:value="filterTacticsId"
            :options="filterOptions"
            placeholder="按策略筛选"
            style="width: 170px"
            clearable
            @update:value="onSearch"
          />
          <n-select
            v-model:value="filterEnabled"
            :options="enabledOptions"
            placeholder="按状态筛选"
            style="width: 130px"
            clearable
            @update:value="onSearch"
          />
        </div>

        <div class="header-right button-actions">
          <n-button type="primary" size="small" class="glow-button" @click="openIpModal()">
            新增 IP 设备
          </n-button>
          <n-button size="small" @click="openImportModal()">
            <template #icon>
              <n-icon><CloudUploadOutline /></n-icon>
            </template>
            Excel 导入
          </n-button>
          <n-button size="small" @click="downloadTemplate()">
            <template #icon>
              <n-icon><DownloadOutline /></n-icon>
            </template>
            下载模板
          </n-button>
          <n-button size="small" @click="handleExport()">
            <template #icon>
              <n-icon><CloudDownloadOutline /></n-icon>
            </template>
            Excel 导出
          </n-button>
          <n-button size="small" class="btn-success-outline" @click="batchUpdateEnabled(true)">
            <template #icon>
              <n-icon><CheckmarkCircleOutline /></n-icon>
            </template>
            批量启用
          </n-button>
          <n-button size="small" class="btn-warning-outline" @click="batchUpdateEnabled(false)">
            <template #icon>
              <n-icon><CloseCircleOutline /></n-icon>
            </template>
            批量停用
          </n-button>
          <n-popconfirm @positive-click="batchDelete">
            <template #trigger>
              <n-button size="small" type="error" :disabled="selectedIps.length === 0" class="btn-error-outline">
                <template #icon>
                  <n-icon><TrashOutline /></n-icon>
                </template>
                批量删除
              </n-button>
            </template>
            确定删除选中的 {{ selectedIps.length }} 个 IP 设备吗？
          </n-popconfirm>
        </div>
      </div>

      <n-data-table
        remote
        :columns="ipColumns"
        :data="ipList"
        :loading="loading"
        :pagination="{
          page: current,
          pageSize: pageSize,
          itemCount: total,
          showSizePicker: true,
          pageSizes: [10, 15, 30, 50],
          prefix: () => `共 ${total} 条设备数据`,
        }"
        :row-key="(row: IP) => row.ipId"
        :flex-height="true"
        class="premium-table"
        style="flex: 1;"
        @update:checked-row-keys="(keys: any[]) => (selectedIps = keys as number[])"
        @update:page="onPageChange"
        @update:page-size="onPageSizeChange"
        striped
        scroll-x="1000"
      />
    </div>
  </div>

  <!-- IP Modal -->
  <n-modal
    v-model:show="ipModalVisible"
    :title="ipModalTitle"
    preset="card"
    style="width: 480px"
    class="glass-modal"
    :segmented="{ content: true }"
  >
    <n-form :model="ipForm" label-width="90px">
      <n-form-item label="设备名称" path="name" required>
        <n-input v-model:value="ipForm.name" placeholder="例如：核心交换机A" />
      </n-form-item>
      <n-form-item label="IP 地址" path="ip" required>
        <n-input v-model:value="ipForm.ip" placeholder="例如：192.168.1.1" />
      </n-form-item>
      <n-form-item label="物理位置" path="position">
        <n-input v-model:value="ipForm.position" placeholder="例如：2号机房4号机架" />
      </n-form-item>
      <n-form-item v-if="!filterTacticsId" label="关联策略组" path="tacticsId" required>
        <n-select
          v-model:value="ipForm.tacticsId"
          :options="tacticsOptions"
          placeholder="选择绑定的探测策略组"
          clearable
        />
      </n-form-item>
      <n-form-item v-else label="关联策略组">
        <n-input :value="currentTacticsName" disabled />
      </n-form-item>
      <n-form-item label="备注信息" path="remark">
        <n-input v-model:value="ipForm.remark" type="textarea" placeholder="填写设备其他备注细节" />
      </n-form-item>
      <n-form-item label="立即启用" path="enabled">
        <n-switch v-model:value="ipForm.enabled" />
      </n-form-item>
      <n-space justify="end" style="margin-top: 10px;">
        <n-button @click="ipModalVisible = false">取消</n-button>
        <n-button type="primary" class="glow-button" @click="saveIp">保存设备</n-button>
      </n-space>
    </n-form>
  </n-modal>

  <!-- Tactics Modal -->
  <n-modal
    v-model:show="tacticsModalVisible"
    :title="tacticsModalTitle"
    preset="card"
    style="width: 480px"
    class="glass-modal"
    :segmented="{ content: true }"
  >
    <n-form
      ref="tacticsFormRef"
      :model="tacticsForm"
      :rules="tacticsRules"
      label-width="120px"
    >
      <n-form-item label="策略名称" path="name">
        <n-input v-model:value="tacticsForm.name" placeholder="例如：高频核心监测" />
      </n-form-item>
      <n-form-item label="检测间隔(ms)" path="intervalMs">
        <n-input-number
          v-model:value="tacticsForm.intervalMs"
          :min="1000"
          placeholder="检测周期频率，默认 60000"
          style="width: 100%"
        />
      </n-form-item>
      <n-form-item label="超时时间(ms)" path="timeoutMs">
        <n-input-number
          v-model:value="tacticsForm.timeoutMs"
          :min="100"
          placeholder="单次 Ping 超时时间，默认 3000"
          style="width: 100%"
        />
      </n-form-item>
      <n-form-item label="不稳定阈值(ms)" path="unstableMs">
        <n-input-number
          v-model:value="tacticsForm.unstableMs"
          :min="0"
          :max="tacticsForm.timeoutMs"
          placeholder="Ping 延迟超过该值即判定为不稳定，默认 0"
          style="width: 100%"
        />
      </n-form-item>
      <n-form-item label="立即启用" path="enabled">
        <n-switch v-model:value="tacticsForm.enabled" />
      </n-form-item>
      <n-space justify="end" style="margin-top: 10px;">
        <n-button @click="tacticsModalVisible = false">取消</n-button>
        <n-button type="primary" class="glow-button" @click="saveTactics">保存配置</n-button>
      </n-space>
    </n-form>
  </n-modal>

  <!-- Import Modal -->
  <n-modal
    v-model:show="importModalVisible"
    title="批量导入 IP 设备"
    preset="card"
    style="width: 480px"
    class="glass-modal"
    :segmented="{ content: true }"
  >
    <n-form label-width="100px">
      <n-form-item label="绑定策略组" required>
        <n-select
          v-model:value="importTacticsId"
          :options="tacticsOptions"
          placeholder="选择要导入的策略组"
          clearable
        />
      </n-form-item>
      <n-form-item label="Excel 文件" required>
        <n-upload
          v-model:file-list="importFileList"
          :default-upload="false"
          accept=".xlsx,.xls"
          :max="1"
        >
          <n-button>选择文件</n-button>
        </n-upload>
      </n-form-item>
      <n-alert type="info" :show-icon="true" class="info-alert">
        Excel 格式：第 1 行为表头，列顺序为 [设备名称、IP 地址、物理位置、备注]。导入后默认直接启用。
        <div style="margin-top: 8px;">
          <n-button size="tiny" text type="primary" @click="downloadTemplate">
            <template #icon>
              <n-icon><DownloadOutline /></n-icon>
            </template>
            下载导入模板
          </n-button>
        </div>
      </n-alert>
      <n-space justify="end" style="margin-top: 16px;">
        <n-button @click="importModalVisible = false">取消</n-button>
        <n-button type="primary" class="glow-button" @click="handleImport">开始导入</n-button>
      </n-space>
    </n-form>
  </n-modal>
</template>

<style scoped>
.manage-container {
  height: calc(100vh - 104px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  gap: 16px;
}

.tactics-section {
  flex-shrink: 0;
}

.ip-section {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 200px;
}

.page-title {
  font-size: 22px;
  font-weight: 700;
  margin-bottom: 8px;
  background: linear-gradient(135deg, #1f2329 0%, #4b5563 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  flex-shrink: 0;
}
.dark .page-title {
  background: linear-gradient(135deg, #f3f4f6 0%, #9ca3af 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

/* Glassmorphism System Cards */
.glass-card {
  background: rgba(255, 255, 255, 0.7);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.4);
  border-radius: 12px;
  box-shadow: 0 8px 32px 0 rgba(31, 38, 135, 0.04);
  padding: 20px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.dark .glass-card {
  background: rgba(24, 24, 28, 0.75);
  backdrop-filter: blur(16px);
  border: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.2);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 18px;
}
.section-title {
  font-size: 16px;
  font-weight: 700;
  margin-right: 8px;
}
.section-subtitle {
  font-size: 12px;
  opacity: 0.55;
  display: block;
  margin-top: 2px;
}

.glow-button {
  background: linear-gradient(135deg, #3b82f6 0%, #2563eb 100%);
  border: none;
  transition: all 0.3s ease;
}
.glow-button:hover {
  box-shadow: 0 0 12px rgba(37, 99, 235, 0.4);
}

/* Tactics scroll view */
.tactics-scroll {
  display: flex;
  gap: 14px;
  overflow-x: auto;
  padding: 4px 2px 12px 2px;
}
.tactics-scroll::-webkit-scrollbar {
  height: 5px;
}
.tactics-scroll::-webkit-scrollbar-track {
  background: transparent;
}
.tactics-scroll::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.1);
  border-radius: 4px;
}
.dark .tactics-scroll::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.15);
}

.t-card {
  flex: 0 0 auto;
  width: 250px;
  background: rgba(255, 255, 255, 0.85);
  border: 1px solid rgba(0, 0, 0, 0.05);
  border-radius: 10px;
  padding: 16px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}
.dark .t-card {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.04);
}

.t-card:hover, .t-card.active {
  transform: translateY(-3px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.06);
  border-color: rgba(59, 130, 246, 0.5);
}
.dark .t-card:hover, .dark .t-card.active {
  background: rgba(255, 255, 255, 0.04);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
}

.t-card.active {
  background: rgba(59, 130, 246, 0.04);
  border-width: 2px;
}
.dark .t-card.active {
  background: rgba(59, 130, 246, 0.08);
}

.t-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}
.t-name {
  font-weight: 700;
  font-size: 15px;
}

.t-card-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
  margin-bottom: 14px;
}
.meta-item {
  display: flex;
  justify-content: space-between;
}
.meta-item .label {
  opacity: 0.55;
}
.meta-item .val {
  font-weight: 600;
  font-family: monospace;
}

.t-card-footer {
  border-top: 1px solid rgba(0, 0, 0, 0.04);
  padding-top: 10px;
  display: flex;
  justify-content: flex-end;
}
.dark .t-card-footer {
  border-top-color: rgba(255, 255, 255, 0.04);
}

.text-btn {
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  opacity: 0.75;
  transition: opacity 0.2s;
}
.text-btn:hover {
  opacity: 1;
}
.text-btn.edit { color: #3b82f6; }
.text-btn.delete { color: #d03050; }

.empty-tactics {
  width: 100%;
  display: flex;
  justify-content: center;
  padding: 20px 0;
}

/* Inline filters and table actions */
.inline-filters {
  display: flex;
  align-items: center;
  gap: 12px;
}
.button-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Outline button styling */
.btn-success-outline:hover { color: #18a058 !important; border-color: #18a058 !important; }
.btn-warning-outline:hover { color: #f0a020 !important; border-color: #f0a020 !important; }
.btn-error-outline:hover { color: #d03050 !important; border-color: #d03050 !important; }

/* Table enhancements */
.ip-code {
  font-family: monospace;
  background: rgba(0, 0, 0, 0.04);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
}
.dark .ip-code {
  background: rgba(255, 255, 255, 0.08);
}
.premium-table :deep(.n-data-table-th) {
  font-weight: 700;
}
.premium-table :deep(.n-data-table-tr:hover) {
  background-color: rgba(59, 130, 246, 0.02) !important;
}
.dark .premium-table :deep(.n-data-table-tr:hover) {
  background-color: rgba(255, 255, 255, 0.01) !important;
}

.btn-action {
  font-weight: 600;
  transition: transform 0.2s;
}
.btn-action:hover {
  transform: scale(1.05);
}

.glass-modal {
  backdrop-filter: blur(10px);
}
.info-alert {
  font-size: 12px;
  margin-top: 10px;
  border-radius: 8px;
}
</style>
