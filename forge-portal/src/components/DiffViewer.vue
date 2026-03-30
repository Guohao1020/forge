<script setup lang="ts">
import type { TaskStepVO } from '@/api/types'

const props = defineProps<{ steps: TaskStepVO[] }>()

interface DiffEntry {
  filePath: string
  original: string
  modified: string
}

function extractDiffs(): DiffEntry[] {
  const diffs: DiffEntry[] = []
  for (const step of props.steps) {
    if (step.outputSnapshot && (step.stepType === 'CODE_FIX' || step.stepType === 'CODE_REVIEW')) {
      try {
        const parsed = JSON.parse(step.outputSnapshot)
        if (Array.isArray(parsed)) {
          for (const f of parsed) {
            diffs.push({
              filePath: f.filePath,
              original: f.originalContent || '(new file)',
              modified: f.content || ''
            })
          }
        }
      } catch {
        // not JSON
      }
    }
  }
  return diffs
}
</script>

<template>
  <div>
    <div v-for="diff in extractDiffs()" :key="diff.filePath" style="margin-bottom: 24px;">
      <h4>{{ diff.filePath }}</h4>
      <div style="display: flex; gap: 8px;">
        <div style="flex: 1;">
          <div style="background: #fff1f0; padding: 4px 8px; font-size: 12px; font-weight: bold;">原始</div>
          <pre style="background: #fff1f0; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 400px;">{{ diff.original }}</pre>
        </div>
        <div style="flex: 1;">
          <div style="background: #f6ffed; padding: 4px 8px; font-size: 12px; font-weight: bold;">修改后</div>
          <pre style="background: #f6ffed; padding: 12px; font-size: 12px; line-height: 1.6; overflow: auto; max-height: 400px;">{{ diff.modified }}</pre>
        </div>
      </div>
    </div>
    <a-empty v-if="extractDiffs().length === 0" description="暂无 Diff 数据" />
  </div>
</template>
