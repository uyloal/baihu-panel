import http from 'node:http';
import https from 'node:https';
import { URL } from 'node:url';
import type { TaskItem, TaskLogResult } from './types.js';

function request<T = any>(urlStr: string, method = 'GET', data: any = null): Promise<T> {
  const token = process.env.BHPKG_OPENAPI_TOKEN || process.env.OPENAPI_TOKEN || process.env.BHPKG_NOTIFY_TOKEN;
  if (!token) {
    throw new Error('没有正确配置或缺少 BHPKG_OPENAPI_TOKEN 环境变量。请在白虎面板设置中配置 OpenAPI Token。');
  }

  const parsedUrl = new URL(urlStr);
  const protocol = parsedUrl.protocol === 'https:' ? https : http;

  let payload = '';
  const headers: Record<string, string | number> = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`
  };

  if (data !== null) {
    payload = JSON.stringify(data);
    headers['Content-Length'] = Buffer.byteLength(payload);
  }

  const options = {
    hostname: parsedUrl.hostname,
    port: parsedUrl.port || (parsedUrl.protocol === 'https:' ? 443 : 80),
    path: parsedUrl.pathname + (parsedUrl.search || ''),
    method: method,
    headers: headers
  };

  return new Promise((resolve, reject) => {
    const req = protocol.request(options, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => (body += chunk));
      res.on('end', () => {
        if (res.statusCode && res.statusCode >= 200 && res.statusCode < 300) {
          try {
            const parsed = body ? JSON.parse(body) : {};
            if (parsed && typeof parsed === 'object' && parsed.code !== undefined && parsed.code !== 200) {
              reject(new Error(`请求失败 [${parsed.code}]: ${parsed.msg || parsed.message || '未知错误'}`));
            } else {
              resolve(parsed.data !== undefined ? parsed.data : parsed);
            }
          } catch {
            resolve(body as unknown as T);
          }
        } else {
          let errMsg = body;
          try {
            const parsed = JSON.parse(body);
            errMsg = parsed.msg || parsed.message || body;
          } catch {}
          reject(new Error(`请求失败 [${res.statusCode}]: ${errMsg}`));
        }
      });
    });

    req.on('error', (e) => reject(e));
    if (payload) {
      req.write(payload);
    }
    req.end();
  });
}

function getBaseUrl(): string {
  const url = process.env.BHPKG_OPENAPI_URL || process.env.OPENAPI_URL;
  if (url) {
    if (url.endsWith('/env')) return url.slice(0, -4);
    if (url.endsWith('/env/')) return url.slice(0, -5);
    return url;
  }

  const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
  const targets = ['/api/v1/notify/send/', '/api/v1/notify/send', '/api/v1/notify/', '/api/v1/notify'];
  for (const target of targets) {
    if (notifyUrl.includes(target)) {
      return notifyUrl.replace(target, '/open2api/v1');
    }
  }
  return 'http://127.0.0.1:8052/open2api/v1';
}

/**
 * 获取所有任务列表
 */
export async function getTasks(): Promise<TaskItem[]> {
  const url = `${getBaseUrl()}/tasks`;
  return await request<TaskItem[]>(url, 'GET');
}

/**
 * 获取单个任务
 */
export async function getTask(id: string): Promise<TaskItem> {
  const url = `${getBaseUrl()}/tasks/${id}`;
  return await request<TaskItem>(url, 'GET');
}

/**
 * 更新任务配置
 */
export async function updateTask(id: string, params: Partial<TaskItem>): Promise<TaskItem> {
  const url = `${getBaseUrl()}/tasks/${id}`;
  return await request<TaskItem>(url, 'PUT', params);
}

/**
 * 删除指定任务
 */
export async function deleteTask(id: string): Promise<boolean> {
  const url = `${getBaseUrl()}/tasks/${id}`;
  await request(url, 'DELETE');
  return true;
}

/**
 * 触发运行指定任务
 */
export async function executeTask(id: string): Promise<{ success: boolean; log_id: string }> {
  const url = `${getBaseUrl()}/execute/task/${id}`;
  return await request<{ success: boolean; log_id: string }>(url, 'POST');
}

/**
 * 停止正在运行的任务
 */
export async function stopTask(logId: string): Promise<{ success: boolean }> {
  const url = `${getBaseUrl()}/tasks/stop/${logId}`;
  return await request<{ success: boolean }>(url, 'POST');
}

/**
 * 获取最近的任务执行结果列表
 */
export async function getLastResults(): Promise<TaskLogResult[]> {
  const url = `${getBaseUrl()}/execute/results`;
  return await request<TaskLogResult[]>(url, 'GET');
}
