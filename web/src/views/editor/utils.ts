import { FILE_RUNNERS } from '@/constants'

export function buildExecutionCommand(
  selectedFile: string,
  baseDir: string
): string {
  const parts = selectedFile.split('/')
  const fileName = parts.pop() || selectedFile
  const dirPath = parts.join('/')
  const ext = fileName.split('.').pop()?.toLowerCase() || ''
  const runner = FILE_RUNNERS[ext] || 'node'

  const cmd = runner ? `${runner} ${fileName}` : `./${fileName}`
  return `cd ${baseDir}${dirPath ? '/' + dirPath : ''} && ${cmd}`
}
