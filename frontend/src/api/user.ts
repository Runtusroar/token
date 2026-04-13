import client from './client';

export const userAPI = {
  getProfile: () => client.get('/user/profile'),

  changePassword: (oldPassword: string, newPassword: string) =>
    client.post('/user/password', { old_password: oldPassword, new_password: newPassword }),

  getDashboard: () => client.get('/user/dashboard'),

  listApiKeys: () => client.get('/user/keys'),

  createApiKey: (name: string) => client.post('/user/keys', { name }),

  deleteApiKey: (id: number) => client.delete(`/user/keys/${id}`),

  updateApiKey: (id: number, data: Record<string, unknown>) =>
    client.put(`/user/keys/${id}`, data),

  listLogs: (page: number, pageSize: number) =>
    client.get('/user/logs', { params: { page, page_size: pageSize } }),

  listBalanceLogs: (page: number, pageSize: number) =>
    client.get('/user/balance/logs', { params: { page, page_size: pageSize } }),

  redeem: (code: string) => client.post('/user/redeem', { code }),
};
