export interface NotifyOptions {
  format?: 'text' | 'markdown' | 'html';
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

export declare function notify(
  title: string,
  content?: string,
  options?: NotifyOptions | 'text' | 'markdown' | 'html' | string
): Promise<boolean>;

export declare function getEnvs(): Promise<EnvironmentVariable[]>;
export declare function getEnv(name: string): Promise<EnvironmentVariable | null>;
export declare function addEnvs(
  envsList: Array<Partial<EnvironmentVariable> & { name: string; value: string }>
): Promise<EnvironmentVariable[]>;
export declare function addEnv(
  name: string,
  value: string,
  remark?: string,
  type?: 'normal' | 'secret',
  hidden?: boolean,
  enabled?: boolean
): Promise<EnvironmentVariable>;
export declare function updateEnv(
  id: string,
  name?: string,
  value?: string,
  remark?: string,
  type?: 'normal' | 'secret',
  hidden?: boolean,
  enabled?: boolean
): Promise<EnvironmentVariable>;
export declare function deleteEnvs(ids: string[]): Promise<boolean>;
export declare function deleteEnv(id: string): Promise<boolean>;

export declare function getTasks(): Promise<TaskItem[]>;
export declare function getTask(id: string): Promise<TaskItem>;
export declare function updateTask(id: string, params: Partial<TaskItem>): Promise<TaskItem>;
export declare function deleteTask(id: string): Promise<boolean>;
export declare function executeTask(id: string): Promise<{ success: boolean; log_id: string }>;
export declare function stopTask(logId: string): Promise<{ success: boolean }>;
export declare function getLastResults(): Promise<TaskLogResult[]>;

declare const baihu: {
  notify: typeof notify;
  getEnvs: typeof getEnvs;
  getEnv: typeof getEnv;
  addEnvs: typeof addEnvs;
  addEnv: typeof addEnv;
  updateEnv: typeof updateEnv;
  deleteEnvs: typeof deleteEnvs;
  deleteEnv: typeof deleteEnv;
  getTasks: typeof getTasks;
  getTask: typeof getTask;
  updateTask: typeof updateTask;
  deleteTask: typeof deleteTask;
  executeTask: typeof executeTask;
  stopTask: typeof stopTask;
  getLastResults: typeof getLastResults;
};

export default baihu;
