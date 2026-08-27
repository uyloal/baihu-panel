/**
 * 白虎面板内置 SDK 类型声明
 */

export interface NotifyOptions {
  /** 消息渲染格式: 默认 text */
  format?: 'text' | 'markdown' | 'html';
  /** 指定推送渠道 ID (留空则推送到默认绑定渠道) */
  channel_id?: string;
  channelId?: string;
}

export interface EnvironmentVariable {
  id: string;
  name: string;
  value: string;
  remark?: string;
  type?: 'normal' | 'secret';
  hidden?: boolean;
  enabled?: boolean;
}

export interface TaskItem {
  id: string;
  name: string;
  command: string;
  schedule: string;
  enabled: boolean;
  status?: string;
  work_dir?: string;
  timeout?: number;
  remark?: string;
  last_run?: string;
  next_run?: string;
}

export interface TaskLogResult {
  id: string;
  task_id: string;
  status: string;
  duration: number;
  exit_code: number;
  start_time: string;
  end_time: string;
}
