import client from './client';

export const adminAPI = {
  getDashboard: () => client.get('/admin/dashboard'),

  getDailyStats: (days = 30) =>
    client.get('/admin/daily-stats', { params: { days } }),

  listUsers: (page: number, pageSize: number, search?: string) =>
    client.get('/admin/users', { params: { page, page_size: pageSize, search } }),

  updateUser: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/users/${id}`, data),

  topUp: (userId: number, amount: number) =>
    client.post(`/admin/users/${userId}/topup`, { amount }),

  userBalanceLogs: (userId: number, page: number, pageSize: number) =>
    client.get(`/admin/users/${userId}/balance-logs`, { params: { page, page_size: pageSize } }),

  userRequestLogs: (userId: number, page: number, pageSize: number) =>
    client.get(`/admin/users/${userId}/request-logs`, { params: { page, page_size: pageSize } }),

  userDailyStats: (userId: number, days = 30) =>
    client.get(`/admin/users/${userId}/daily-stats`, { params: { days } }),

  listChannels: () => client.get('/admin/channels'),

  createChannel: (data: Record<string, unknown>) =>
    client.post('/admin/channels', data),

  updateChannel: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/channels/${id}`, data),

  deleteChannel: (id: number) => client.delete(`/admin/channels/${id}`),

  testChannel: (id: number) => client.post(`/admin/channels/${id}/test`),

  listModels: () => client.get('/admin/models'),

  createModel: (data: Record<string, unknown>) =>
    client.post('/admin/models', data),

  updateModel: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/models/${id}`, data),

  deleteModel: (id: number) => client.delete(`/admin/models/${id}`),

  listRedeemCodes: (page: number, pageSize: number) =>
    client.get('/admin/redeem-codes', { params: { page, page_size: pageSize } }),

  createRedeemCodes: (data: Record<string, unknown>) =>
    client.post('/admin/redeem-codes', data),

  updateRedeemCode: (id: number, data: Record<string, unknown>) =>
    client.put(`/admin/redeem-codes/${id}`, data),

  listLogs: (page: number, pageSize: number, userId?: number, model?: string) =>
    client.get('/admin/logs', { params: { page, page_size: pageSize, user_id: userId, model } }),

  getSettings: () => client.get('/admin/settings'),

  updateSettings: (data: Record<string, unknown>) =>
    client.put('/admin/settings', data),
};
