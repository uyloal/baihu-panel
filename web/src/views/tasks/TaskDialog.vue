<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { ScrollArea } from '@/components/ui/scroll-area'
import DirTreeSelect from '@/components/DirTreeSelect.vue'
import { Plus, X, ChevronDown, Search, Terminal, Zap, Lock, Variable } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import { api, type Task, type EnvVar } from '@/api'
import { PATHS, TRIGGER_TYPE } from '@/constants'
import { toast } from 'vue-sonner'

import TaskNotificationConfig from './components/TaskNotificationConfig.vue'
import TaskAdvancedConfig from './components/TaskAdvancedConfig.vue'
import TaskCronConfig from './components/TaskCronConfig.vue'
import TaskTagsConfig from './components/TaskTagsConfig.vue'

const props = defineProps<{
  open: boolean
  task?: Partial<Task>
  isEdit: boolean
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  'saved': []
}>()

const form = ref<Partial<Task>>({})
const allEnvVars = ref<EnvVar[]>([])
const selectedEnvIds = ref<string[]>([])
const selectedTriggerType = ref<string>('cron')
const envSearchQuery = ref('')
const currentWorkDir = ref('')
const commentToTaskEnabled = ref(false)
const allEnvsEnabled = ref(false)
const scriptsDir = ref<string>(PATHS.SCRIPTS_DIR)

function onAllEnvsChange(val: boolean) {
  allEnvsEnabled.value = val
}

const filteredEnvVars = computed(() => {
  return allEnvVars.value.filter((env: EnvVar) => {
    const q = envSearchQuery.value.toLowerCase()
    const matchSearch = !q || 
      env.name.toLowerCase().includes(q) || 
      (env.remark && env.remark.toLowerCase().includes(q)) ||
      (env.tags && env.tags.toLowerCase().includes(q))
    const notSelected = !selectedEnvIds.value.includes(env.id)
    return matchSearch && notSelected
  })
})

const selectedEnvs = computed(() => {
  return selectedEnvIds.value
    .map((id: string) => allEnvVars.value.find((e: EnvVar) => e.id === id))
    .filter((e): e is EnvVar => e !== undefined)
})

const notificationConfigRef = ref<InstanceType<typeof TaskNotificationConfig> | null>(null)

watch(() => props.open, async (val: boolean) => {
  if (val) {
    form.value = {
      retry_count: props.task?.retry_count ?? 0,
      retry_interval: props.task?.retry_interval ?? 0,
      random_range: props.task?.random_range ?? 0,
      timeout: props.task?.timeout ?? 30,
      pin_type: props.task?.pin_type ?? 'none',
      pre_command: props.task?.pre_command ?? '',
      post_command: props.task?.post_command ?? '',
      ...props.task
    }
    
    try {
      let configStr = props.task?.config || '{}'
      const parsed = JSON.parse(configStr)
      if (parsed && typeof parsed === 'object') {
        allEnvsEnabled.value = !!parsed['$task_all_envs']
        commentToTaskEnabled.value = !!parsed['$task_comment_to_task']
      } else {
        allEnvsEnabled.value = false
        commentToTaskEnabled.value = false
      }
    } catch {
      allEnvsEnabled.value = false
      commentToTaskEnabled.value = false
    }

    if (props.task?.envs) {
      selectedEnvIds.value = props.task.envs.split(',').map((s: string) => s.trim()).filter(Boolean)
    } else {
      selectedEnvIds.value = []
    }

    selectedTriggerType.value = props.task?.trigger_type || TRIGGER_TYPE.CRON
    envSearchQuery.value = ''
    await loadData()
    currentWorkDir.value = normalizeLocalWorkDirForDisplay(props.task?.work_dir)

    await notificationConfigRef.value?.loadConfig(props.isEdit ? props.task?.id : undefined)
  }
})

async function loadData() {
  try {
    const [envs, paths] = await Promise.all([
      api.env.all(),
      api.settings.getPaths().catch(() => ({ scripts_dir: PATHS.SCRIPTS_DIR }))
    ])
    allEnvVars.value = envs
    scriptsDir.value = paths?.scripts_dir || PATHS.SCRIPTS_DIR
  } catch { /* ignore */ }
}

function addEnv(id: string) {
  if (!selectedEnvIds.value.includes(id)) {
    selectedEnvIds.value.push(id)
  }
}

function removeEnv(id: string) {
  selectedEnvIds.value = selectedEnvIds.value.filter((envId: string) => envId !== id)
}

function normalizeLocalWorkDirForDisplay(workDir?: string | null): string {
  if (!workDir) return ''
  if (workDir === PATHS.SCRIPTS_DIR_PLACEHOLDER) return ''
  if (workDir.startsWith(`${PATHS.SCRIPTS_DIR_PLACEHOLDER}/`)) {
    return workDir.slice(PATHS.SCRIPTS_DIR_PLACEHOLDER.length + 1)
  }
  const base = scriptsDir.value || PATHS.SCRIPTS_DIR
  if (workDir === base) return ''
  if (workDir.startsWith(`${base}/`)) {
    return workDir.slice(base.length + 1)
  }
  return workDir
}

function encodeLocalWorkDir(workDir?: string | null): string {
  const value = workDir?.trim() || ''
  if (!value) return PATHS.SCRIPTS_DIR_PLACEHOLDER
  if (value === PATHS.SCRIPTS_DIR_PLACEHOLDER || value.startsWith(`${PATHS.SCRIPTS_DIR_PLACEHOLDER}/`)) {
    return value
  }
  const base = scriptsDir.value || PATHS.SCRIPTS_DIR
  if (value === base) return PATHS.SCRIPTS_DIR_PLACEHOLDER
  if (value.startsWith(`${base}/`)) {
    return `${PATHS.SCRIPTS_DIR_PLACEHOLDER}/${value.slice(base.length + 1)}`
  }
  return `${PATHS.SCRIPTS_DIR_PLACEHOLDER}/${value.replace(/^\/+/, '')}`
}

async function save() {
  try {
    form.value.envs = selectedEnvIds.value.join(',')
    form.value.type = 'task'
    form.value.trigger_type = selectedTriggerType.value

    let config: Record<string, any> = {}
    if (form.value.config) {
      try {
        const parsed = JSON.parse(form.value.config)
        if (parsed && typeof parsed === 'object') {
          config = parsed
        }
      } catch {
        config = {}
      }
    }

    config['$task_all_envs'] = !!allEnvsEnabled.value
    config['$task_comment_to_task'] = !!commentToTaskEnabled.value
    form.value.config = JSON.stringify(config)
    form.value.work_dir = encodeLocalWorkDir(currentWorkDir.value)

    if (props.isEdit && form.value.id) {
      const task = await api.tasks.update(form.value.id, form.value)
      await notificationConfigRef.value?.saveConfig(task.id)
      toast.success('任务已更新')
    } else {
      const task = await api.tasks.create(form.value)
      await notificationConfigRef.value?.saveConfig(task.id)
      toast.success('任务已创建')
    }
    emit('update:open', false)
    emit('saved')
  } catch (error: any) {
    toast.error('保存失败', {
      description: error.message || '未知错误'
    })
  }
}
</script>

<template>
  <Dialog :open="open" @update:open="emit('update:open', $event)">
    <DialogContent class="max-w-[95vw] sm:max-w-[600px] xl:max-w-[850px] p-0 overflow-hidden border-none bg-background shadow-2xl transition-all duration-300" style="text-rendering: optimizeLegibility;" @openAutoFocus.prevent @pointerDownOutside.prevent>
      <div class="flex flex-col max-h-[85vh]">
        <DialogHeader class="px-6 pr-12 pt-6 pb-2 shrink-0 border-b border-muted/50">
          <DialogTitle class="text-xl font-bold py-2">
            {{ isEdit ? '编辑任务' : '新建任务' }}
          </DialogTitle>
        </DialogHeader>

        <ScrollArea class="flex-1 min-h-0 px-6">
          <div class="space-y-10 py-6 pb-10">
            <!-- 基本信息 Section -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 mb-2">
                <div class="h-4 w-1 bg-primary rounded-full shadow-sm shadow-primary/20" />
                <h3 class="text-sm font-bold text-foreground/90">基本信息</h3>
              </div>
              <div class="grid gap-5 pl-3 border-l border-muted">
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">任务名称</Label>
                  <Input v-model="form.name" placeholder="输入任务描述性名称" :class="cn('sm:col-span-3 h-9 bg-muted/20 border-muted-foreground/15 transition-all focus:bg-background/50', form.name ? 'text-sm font-medium' : 'text-[11px] font-normal')" />
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">任务备注</Label>
                  <Input v-model="form.remark" placeholder="输入任务备注信息 (可选)" :class="cn('sm:col-span-3 h-9 bg-muted/20 border-muted-foreground/15 transition-all focus:bg-background/50', form.remark ? 'text-sm font-medium' : 'text-[11px] font-normal')" />
                </div>
                <TaskTagsConfig v-model="form.tags" />
                
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">触发方式</Label>
                  <div class="sm:col-span-3 min-w-0">
                    <Select v-model="selectedTriggerType">
                      <SelectTrigger class="h-9 bg-muted/20 border-muted-foreground/15 px-2 sm:px-3 text-xs sm:text-sm min-w-0">
                        <SelectValue class="truncate" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem :value="TRIGGER_TYPE.CRON" class="text-xs sm:text-sm">⏳ 定时周期</SelectItem>
                        <SelectItem :value="TRIGGER_TYPE.BAIHU_STARTUP" class="text-xs sm:text-sm">🚀 系统启动</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>
            </section>

            <!-- 执行配置 Section -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 mb-2">
                <div class="h-4 w-1 bg-primary rounded-full shadow-sm shadow-primary/20" />
                <h3 class="text-sm font-bold text-foreground/90">执行配置</h3>
              </div>
              <div class="grid gap-5 pl-3 border-l border-muted">
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">前置指令</Label>
                  <div class="sm:col-span-3 relative"><Input v-model="form.pre_command" placeholder="执行主命令前运行的指令 (可选)" :class="cn('h-9 bg-muted/20 border-muted-foreground/15 transition-all focus:bg-background/50 pr-10', form.pre_command ? 'font-mono text-sm tracking-tight font-medium' : 'text-[11px] font-normal')" /><Zap class="absolute right-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground opacity-40 pointer-events-none" /></div>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">核心命令</Label>
                  <div class="sm:col-span-3 relative"><Input v-model="form.command" placeholder="例如: node index.js 或 ts-node test.ts" :class="cn('h-9 bg-muted/20 border-muted-foreground/15 transition-all focus:bg-background/50 pr-10', form.command ? 'font-mono text-sm tracking-tight font-medium' : 'text-[11px] font-normal')" /><Terminal class="absolute right-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground opacity-40 pointer-events-none" /></div>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">后置指令</Label>
                  <div class="sm:col-span-3 relative"><Input v-model="form.post_command" placeholder="主命令执行后运行的指令 (可选)" :class="cn('h-9 bg-muted/20 border-muted-foreground/15 transition-all focus:bg-background/50 pr-10', form.post_command ? 'font-mono text-sm tracking-tight font-medium' : 'text-[11px] font-normal')" /><Zap class="absolute right-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground opacity-40 pointer-events-none" /></div>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">工作目录</Label>
                  <div class="sm:col-span-3"><DirTreeSelect v-model="currentWorkDir" class="h-9" /></div>
                </div>
                <div class="grid grid-cols-1 sm:grid-cols-4 items-center gap-3 pb-1">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold">变量注入</Label>
                  <div class="sm:col-span-3"><div class="flex items-center space-x-2 bg-muted/10 px-3 py-1.5 rounded-full border border-muted-foreground/10 w-fit"><Switch :model-value="allEnvsEnabled" @update:model-value="onAllEnvsChange" id="all-envs" class="scale-90" /><Label for="all-envs" class="text-[11px] font-medium cursor-pointer">全量注入</Label></div></div>
                </div>
                <div v-if="!allEnvsEnabled" class="grid grid-cols-1 sm:grid-cols-4 items-start gap-3">
                  <Label class="sm:text-right text-xs text-foreground/70 uppercase tracking-wider font-bold pt-2.5">按需包含</Label>
                  <div class="sm:col-span-3 space-y-2">
                    <Popover>
                      <PopoverTrigger as-child>
                        <Button variant="outline" class="w-full justify-between h-9 bg-muted/10 border-muted-foreground/15 text-xs rounded-xl">
                          <span class="text-muted-foreground">选择关联的环境变量...</span>
                          <ChevronDown class="h-4 w-4 opacity-30" />
                        </Button>
                      </PopoverTrigger>
                      <PopoverContent class="p-0 w-[calc(100vw-32px)] sm:w-[480px] md:w-[540px] max-h-[480px] overflow-hidden rounded-2xl shadow-2xl border-primary/10 transition-all duration-300" 
                        align="center" :align-offset="0" :side-offset="12">
                        <div class="px-4 py-3.5 border-b bg-muted/20 backdrop-blur-md sticky top-0 z-10">
                          <div class="relative group">
                            <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground group-focus-within:text-primary transition-colors duration-300" />
                            <Input v-model="envSearchQuery" placeholder="输入关键字搜索变量名、备注或标签..." 
                              class="pl-9 h-9 bg-background/50 border-primary/10 focus:border-primary/30 transition-all rounded-lg text-[13px]" />
                          </div>
                        </div>
                        <ScrollArea class="h-[320px] px-2 py-1.5 overflow-x-hidden">
                          <div v-if="filteredEnvVars.length === 0" class="py-16 text-center text-xs text-muted-foreground flex flex-col items-center gap-3 animate-in fade-in duration-500">
                            <div class="h-10 w-10 rounded-full bg-muted/30 flex items-center justify-center mb-1">
                               <Search class="h-5 w-5 opacity-20" />
                            </div>
                            未找到符合条件的变量
                          </div>
                          <div v-for="env in filteredEnvVars" :key="env.id" @click.stop="addEnv(env.id)"
                            class="flex flex-col p-2.5 rounded-lg hover:bg-primary/5 cursor-pointer transition-all duration-300 border border-transparent hover:border-primary/10 mb-0.5 group relative">
                            <div class="flex items-center justify-between mb-1">
                              <div class="flex items-center gap-2 min-w-0">
                                <component :is="env.type === 'secret' ? Lock : Variable" 
                                  :class="cn('h-3.5 w-3.5 shrink-0', env.type === 'secret' ? 'text-amber-500' : 'text-blue-500/60')" />
                                <code class="text-[11px] font-mono font-bold bg-muted/60 px-1.5 py-0.5 rounded text-foreground/80 group-hover:bg-primary/10 group-hover:text-primary transition-all truncate border border-transparent group-hover:border-primary/20">
                                  {{ env.name }}
                                </code>
                                <Badge v-if="env.type === 'secret'" variant="outline" class="h-4 px-1.5 text-[9px] border-amber-500/20 bg-amber-500/5 text-amber-600 font-bold uppercase tracking-tight scale-90">
                                  机密
                                </Badge>
                                <div v-if="env.tags" class="flex items-center gap-1 overflow-hidden ml-1">
                                  <span v-for="tag in env.tags.split(',').filter(Boolean).slice(0, 3)" :key="tag" class="truncate text-[9px] leading-none px-1 py-0.5 bg-secondary text-secondary-foreground rounded border">{{ tag }}</span>
                                </div>
                              </div>
                              <div class="flex items-center gap-2 scale-75 opacity-0 group-hover:opacity-100 group-hover:translate-x-0 translate-x-2 transition-all duration-300 shrink-0">
                                <Plus class="h-4 w-4 text-primary" />
                              </div>
                            </div>
                            <div class="text-[10px] text-muted-foreground/50 line-clamp-1 leading-relaxed group-hover:text-muted-foreground transition-colors ml-6 pl-1.5 border-l-2 border-transparent group-hover:border-primary/5 italic truncate">
                              {{ env.remark || "暂无备注" }}
                            </div>
                            <div class="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-0 bg-primary/40 rounded-full group-hover:h-6 transition-all duration-300" />
                          </div>
                        </ScrollArea>
                      </PopoverContent>
                    </Popover>
                    <div v-if="selectedEnvs.length > 0" class="flex flex-wrap gap-2 p-3 rounded-xl bg-muted/10 border border-muted-foreground/10 min-h-12"><div v-for="env in selectedEnvs" :key="env?.id" class="flex items-center gap-1.5 px-2.5 py-1 rounded-full bg-background border border-muted-foreground/15 text-[11px] font-mono font-medium">{{ env?.name }}<X class="h-2.5 w-2.5 cursor-pointer" @click="removeEnv(env!.id)" /></div></div>
                  </div>
                </div>
              </div>
            </section>

            <!-- 调度策略 Section -->
            <section class="space-y-4">
              <div class="flex items-center gap-2 mb-2">
                <div class="h-4 w-1 bg-primary rounded-full shadow-sm shadow-primary/20" />
                <h3 class="text-sm font-bold text-foreground/90">调度策略</h3>
              </div>
              <div class="grid gap-5 pl-3 border-l border-muted">
                <template v-if="selectedTriggerType === TRIGGER_TYPE.CRON">
                  <TaskCronConfig v-model="form.schedule" />
                </template>

                <TaskAdvancedConfig v-model="form" />

              </div>
            </section>
            <TaskNotificationConfig ref="notificationConfigRef" :task-id="isEdit ? task?.id : undefined" />
          </div>
        </ScrollArea>
        <div class="flex items-center justify-between px-6 py-4 bg-muted/20 border-t shrink-0 backdrop-blur-sm">
          <div class="text-[10px] text-muted-foreground/40 italic flex flex-col leading-tight select-none pointer-events-none">
            <span>最后编辑于:</span>
            <span>{{ isEdit ? (form.updated_at || '刚才') : '现在' }}</span>
          </div>
          <div class="flex gap-3">
            <Button variant="ghost" size="sm" class="hover:bg-muted font-medium text-xs px-6" @click="emit('update:open', false)">取消</Button>
            <Button size="sm" class="px-8 font-semibold text-xs shadow-lg shadow-primary/20 bg-primary hover:bg-primary/90" @click="save">确定保存</Button>
          </div>
        </div>
      </div>
    </DialogContent>
  </Dialog>
</template>

<style scoped>
:deep(*) {
  text-rendering: optimizeLegibility;
}
:deep(label) {
  text-rendering: optimizeLegibility;
  letter-spacing: 0.01em;
}
</style>
