import type { ThemeConfig } from 'antd';

const theme: ThemeConfig = {
  token: {
    fontFamily: "'JetBrains Mono', 'LXGW WenKai Mono', 'Consolas', monospace",
    fontSize: 13,
    colorPrimary: '#1a1a1a',
    colorSuccess: '#0a8c2d',
    colorError: '#cc0000',
    borderRadius: 0,
    colorBorder: '#1a1a1a',
    colorBgLayout: '#f5f5f0',
  },
  components: {
    Button: {
      borderRadius: 0,
      borderRadiusLG: 0,
      borderRadiusSM: 0,
    },
    Input: {
      borderRadius: 0,
      borderRadiusLG: 0,
      borderRadiusSM: 0,
    },
    Table: {
      borderRadius: 0,
      borderRadiusLG: 0,
    },
    Card: {
      borderRadius: 0,
      borderRadiusLG: 0,
    },
    Menu: {
      borderRadius: 0,
      borderRadiusLG: 0,
      borderRadiusSM: 0,
    },
  },
};

export default theme;
