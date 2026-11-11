export default {
  reputation: {
    title: '信誉分调度',
    description: '按用户分数在同一分组内筛选账号。与风控、内容审计设置相互独立。',
    nav: '信誉分',
    save: '保存设置',
    saved: '信誉分设置已保存',
    saveFailed: '保存信誉分设置失败',
    loadFailed: '加载信誉分设置失败',
    enabled: '启用信誉分调度',
    enabledHint: '关闭后调度忽略档位，用户列表和账号表单隐藏相关字段，已保存数据保留。',
    initialScore: '注册初始分',
    initialScoreHint: '新用户注册时写入的分数，范围 0–100。',
    tiers: {
      title: '档位',
      description: '账号绑定档位 ID。用户请求按当前分数向下取档，只调度同档账号。',
      add: '添加档位',
      name: '名称',
      score: '分数线',
      empty: '尚未配置档位',
      remove: '删除'
    },
    events: {
      title: '事件',
      description: '分数只由这些事件加减。管理员改分会记审计，不占用事件表。',
      enabled: '启用',
      delta: '扣分',
      thresholdAmount: '满足金额',
      pointsPerThreshold: '每次加分',
      register: '注册',
      registerHint: '写入初始分，不累计加减。',
      content_audit_block: '内容审计拦截',
      content_audit_blockHint: '出网前因内容策略被拒时扣分。',
      own_account_security: '自有账号安全事件',
      own_account_securityHint: 'oauth / setup-token 账号命中安全限制时扣分。',
      upstream_security: '上游安全事件',
      upstream_securityHint: '上游账号命中安全限制时扣分。',
      recharge: '充值',
      rechargeHint: '余额充值按「满足金额加分」累计，余数不计。',
      redeem_balance: '兑换码（余额）',
      redeem_balanceHint: '仅余额类兑换码按「满足额度加分」累计。'
    },
    account: {
      tier: '信誉档位',
      tierHint: '该账号只承接对应该档的用户请求。',
      scoreEnabled: '安全事件扣分',
      scoreEnabledHint: '关闭后该账号触发安全事件不扣用户分。oauth / setup-token 默认开启，上游账号默认关闭。'
    },
    users: {
      score: '信誉分',
      tier: '信誉档位',
      scoreHint: '0–100，保存后档位按分数自动计算。',
      invalidScore: '信誉分必须是 0 到 100 的整数',
      scoreUpdated: '信誉分已更新'
    }
  }
}
