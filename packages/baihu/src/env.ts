import http from 'node:http';
import https from 'node:https';
import { URL } from 'node:url';
import type { EnvironmentVariable } from './types.js';

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

function getEnvsUrl(): string {
  const url = process.env.BHPKG_OPENAPI_URL || process.env.OPENAPI_URL;
  if (url) return url.endsWith('/env') ? url : `${url}/env`;

  const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
  const targets = ['/api/v1/notify/send/', '/api/v1/notify/send', '/api/v1/notify/', '/api/v1/notify'];
  for (const target of targets) {
    if (notifyUrl.includes(target)) {
      return notifyUrl.replace(target, '/open2api/v1/env');
    }
  }
  return 'http://127.0.0.1:8052/open2api/v1/env';
}

/**
 * 获取所有环境变量列表
 */
export async function getEnvs(): Promise<EnvironmentVariable[]> {
  const url = `${getEnvsUrl()}/all`;
  return await request<EnvironmentVariable[]>(url, 'GET');
}

/**
 * 根据变量名获取环境变量，不存在返回 null
 */
export async function getEnv(name: string): Promise<EnvironmentVariable | null> {
  const envs = await getEnvs();
  if (Array.isArray(envs)) {
    for (const env of envs) {
      if (env.name === name) {
        return env;
      }
    }
  }
  return null;
}

/**
 * 批量添加环境变量
 */
export async function addEnvs(envsList: Array<Partial<EnvironmentVariable> & { name: string; value: string }>): Promise<EnvironmentVariable[]> {
  const url = getEnvsUrl();
  const addedEnvs: EnvironmentVariable[] = [];
  for (const env of envsList) {
    if (!env.name || env.value === undefined) {
      throw new Error("环境变量必须包含 'name' 和 'value'");
    }
    const res = await request<EnvironmentVariable>(url, 'POST', env);
    if (res) {
      addedEnvs.push(res);
    }
  }
  return addedEnvs;
}

/**
 * 添加单个环境变量
 */
export async function addEnv(
  name: string,
  value: string,
  remark = '',
  type: 'normal' | 'secret' = 'normal',
  hidden = true,
  enabled = true
): Promise<EnvironmentVariable> {
  const url = getEnvsUrl();
  const payload = {
    name,
    value,
    remark,
    type,
    hidden,
    enabled
  };
  return await request<EnvironmentVariable>(url, 'POST', payload);
}

/**
 * 更新环境变量
 */
export async function updateEnv(
  id: string,
  name?: string,
  value?: string,
  remark?: string,
  type?: 'normal' | 'secret',
  hidden?: boolean,
  enabled?: boolean
): Promise<EnvironmentVariable> {
  const url = `${getEnvsUrl()}/${id}`;
  const payload: Record<string, any> = {};
  if (name !== undefined) payload.name = name;
  if (value !== undefined) payload.value = value;
  if (remark !== undefined) payload.remark = remark;
  if (type !== undefined) payload.type = type;
  if (hidden !== undefined) payload.hidden = hidden;
  if (enabled !== undefined) payload.enabled = enabled;

  return await request<EnvironmentVariable>(url, 'PUT', payload);
}

/**
 * 批量删除环境变量
 */
export async function deleteEnvs(ids: string[]): Promise<boolean> {
  for (const id of ids) {
    await deleteEnv(id);
  }
  return true;
}

/**
 * 删除指定环境变量
 */
export async function deleteEnv(id: string): Promise<boolean> {
  const url = `${getEnvsUrl()}/${id}`;
  await request(url, 'DELETE');
  return true;
}
