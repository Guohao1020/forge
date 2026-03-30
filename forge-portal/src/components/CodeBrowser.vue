<script setup lang="ts">
import { ref } from 'vue'
import type { TaskStepVO } from '@/api/types'

const props = defineProps<{ steps: TaskStepVO[] }>()

interface FileEntry {
  path: string
  content: string
  action: string
}

const selectedFile = ref<FileEntry | null>(null)

function extractFiles(): FileEntry[] {
  const files: FileEntry[] = []
  for (const step of props.steps) {
    if (step.outputSnapshot && step.stepType === 'CODE_GENERATE') {
      try {
        const parsed = JSON.parse(step.outputSnapshot)
        if (Array.isArray(parsed)) {
          for (const f of parsed) {
            files.push({ path: f.filePath, content: f.content, action: f.action || 'CREATE' })
          }
        }
      } catch {
        // not JSON, skip
      }
    }
  }
  return files
}
</script>

<template>
  <div style="display: flex; gap: 16px;">
    <div style="width: 300px; border-right: 1px solid #f0f0f0; padding-right: 16px;">
      <h4>文件列表</h4>
      <a-list size="small" :data-source="extractFiles()">
        <template #renderItem="{ item }">
          <a-list-item
            style="cursor: pointer; padding: 4px 8px;"
            :style="{ background: selectedFile?.path === item.path ? '#e6f7ff' : 'transparent' }"
            @click="selectedFile = item"
          >
            <a-tag :color="item.action === 'CREATE' ? 'green' : 'blue'">
              {{ item.action }}
            </a-tag>
            <span style="font-size: 13px; margin-left: 4px;">{{ item.path }}</span>
          </a-list-item>
        </template>
      </a-list>
      <a-empty v-if="extractFiles().length === 0" description="暂无生成文件" />
    </div>
    <div style="flex: 1; overflow: auto;">
      <template v-if="selectedFile">
        <h4>{{ selectedFile.path }}</h4>
        <pre style="background: #fafafa; padding: 16px; border-radius: 4px; font-size: 13px; line-height: 1.6; overflow: auto; max-height: 600px;">{{ selectedFile.content }}</pre>
      </template>
      <a-empty v-else description="请选择文件查看" />
    </div>
  </div>
</template>
