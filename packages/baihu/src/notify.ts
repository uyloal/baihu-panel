import http from 'node:http';
import https from 'node:https';
import { URL } from 'node:url';
import type { NotifyOptions } from './types.js';

/**
 * 主动触发向管理员推送告警通知
 * @param title 通知标题
 * @param content 通知正文内容
 * @param options 格式选项或渠道 ID (或 'text' | 'markdown' | 'html')
 */
export function notify(
  title: string,
  content?: string,
  options?: NotifyOptions | 'text' | 'markdown' | 'html' | string
): Promise<boolean> {
  return new Promise((resolve) => {
    let format = 'text';
    let channel_id = '';

    if (typeof options === 'string') {
      if (['text', 'markdown', 'html'].includes(options)) {
        format = options;
      } else {
        channel_id = options;
      }
    } else if (typeof options === 'object' && options !== null) {
      format = options.format || 'text';
      channel_id = options.channel_id || options.channelId || '';
    }

    const token = process.env.BHPKG_NOTIFY_TOKEN || process.env.OPENAPI_TOKEN || process.env.BHPKG_OPENAPI_TOKEN;
    const defaultChannel = process.env.BHPKG_NOTIFY_CHANNEL || '';
    const cid = channel_id || defaultChannel;

    if (!token) {
      // 未配置 Token 时静默跳过
      resolve(false);
      return;
    }

    const notifyUrl = process.env.BHPKG_NOTIFY_URL || 'http://127.0.0.1:8052/api/v1/notify/send';
    let parsedUrl: URL;
    try {
      parsedUrl = new URL(notifyUrl);
    } catch {
      resolve(false);
      return;
    }

    const client = parsedUrl.protocol === 'https:' ? https : http;
    const postData = JSON.stringify({
      channel_id: cid,
      title: title || '任务运行通知',
      content: content || '',
      format: format
    });

    const req = client.request(
      {
        hostname: parsedUrl.hostname,
        port: parsedUrl.port || (parsedUrl.protocol === 'https:' ? 443 : 80),
        path: parsedUrl.pathname + parsedUrl.search,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'notify-token': token,
          'Authorization': `Bearer ${token}`,
          'Content-Length': Buffer.byteLength(postData)
        },
        timeout: 5000
      },
      (res) => {
        res.resume();
        resolve(res.statusCode !== undefined && res.statusCode >= 200 && res.statusCode < 300);
      }
    );

    req.on('error', () => {
      resolve(false);
    });

    req.on('timeout', () => {
      req.destroy();
      resolve(false);
    });

    req.write(postData);
    req.end();
  });
}
