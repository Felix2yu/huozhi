---
name: "huozhi-card-ui"
description: "银行卡/信用卡管理页的完整实现模板：AES-GCM 加密存储 + 3D 卡面翻转 UI + 银行名 Combobox。Invoke when building/extending Huozhi card-management features or adding new card-sensitive fields."
---

# Huozhi 银行卡管理 Skill

为货殖记账应用的**银行卡/信用卡**相关功能提供可复用的实现规范。涵盖后端加密存储模型、前端 3D 卡面渲染、银行名下拉、敏感信息按需解密 4 大模块。

---

## 一、后端安全模型

### 敏感字段加密规则

| 字段 | 模型位置 | 存储方式 | 默认是否返回前端 |
|------|---------|---------|--------------|
| EncryptedCardNo | Account | AES-256-GCM + `json:"-"` | ❌ 从不 |
| EncryptedCVV | Account | AES-256-GCM + `json:"-"` | ❌ 从不 |
| CardNo4（尾号 4 位）| Account | 明文 | ✅ |
| ExpireMonth/Year | Account | 明文（便于筛选） | ✅ |
| BankName | Account | 明文 | ✅ |

### 加密工具位置

```
backend/pkg/crypto/crypto.go
├── KeyFromSecret(secret string) []byte   // SHA-256 派生 32 字节 AES key
├── Encrypt(plaintext, key []byte) string // AES-GCM → base64(nonce+ct+tag)
└── Decrypt(ciphertext string, key []byte) (string, error)
```

### 密钥派生

统一用 `config.AppConfig.JWT.Secret`：

```go
key := crypto.KeyFromSecret(config.AppConfig.JWT.Secret)
ct, _ := crypto.Encrypt([]byte(plaintext), key)
```

### Create / Update 加密流程

Create handler（[account.go](file:///Users/yufei/Git/huozhi/backend/internal/handlers/account.go)）：
```go
// 完整卡号：加密 + 自动提取尾号
var encryptedCardNo string
cardNo4 := req.CardNo4
if req.FullCardNo != "" {
    encryptedCardNo = encryptCardNo(req.FullCardNo)
    if cardNo4 == "" {
        cardNo4 = tail4(req.FullCardNo)  // strings.ReplaceAll 去空格后取后 4 位
    }
}
// CVV：非空才加密
var encryptedCVV string
if req.CVV != "" {
    encryptedCVV = encryptCardNo(req.CVV)
}
```

Update handler：**非空才覆盖，空则保持原值**（避免用户更新其他字段时把敏感字段刷空）。

### 按需解密接口

```
GET /api/accounts/:id/full-card  →  { full_card_no, cvv, expire_month, expire_year }
```

- 必须先验证 `id + user_id` 所有权
- `EncryptedCardNo` 和 `EncryptedCVV` **任一为空则整体 NotFound**
- DTO 加字段用 `binding:"omitempty,max=N"`

### 新增敏感字段的 Checklist

1. `models.go` 加 `EncryptedXXX string gorm:"size:128~512" json:"-"`
2. `dto.go` DTO 加输入字段 `XXX string binding:"omitempty,max=N"`
3. Create/Update handler 加加密逻辑
4. GetFullCardNo 返回解密后的值
5. 前端新增表单字段 + 默认脱敏显示

---

## 二、前端 3D 卡面系统

### 核心文件

```
frontend/src/styles/index.css         # 265 行, 从第 101 行开始
frontend/src/pages/cards/CardsPage.tsx
frontend/src/pages/accounts/AccountsPage.tsx  # 银行卡表单 + 脱敏列表
```

### CSS 关键类

| 类名 | 用途 |
|------|------|
| `.card3d-wrap { perspective: 1200px }` | 父元素必须有 perspective |
| `.card3d { transform-style: preserve-3d; aspect-ratio: 1.586/1 }` | 标准银行卡比例 |
| `.card3d.flipped { transform: rotateY(180deg) }` | 翻转状态 |
| `.card3d-face { backface-visibility: hidden; padding: 20px 22px }` | 正反面通用 |
| `.card3d-face.back { transform: rotateY(180deg) }` | 背面反向 |
| `.card3d-bank-{0..7}` | 8 种银行渐变色（hash 分配） |
| `.card3d-credit` | 信用卡黑金磨砂卡面 |
| `.card3d-shine` | 金属光泽扫光动画（8s loop） |
| `.card3d-chip` | 仿真芯片（金色 + 栅格） |
| `.card3d-magstripe` | 黑色磁条 |
| `.card3d-sigstrip` + `.card3d-cvv-box` | CVV 签名条 |

### 银行配色 Hash

```ts
function bankColorIdx(name: string): number {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0;
  return h % 8;
}
```

### 复用 CardFace 组件签名

```tsx
interface CardFaceData {
  id: number;
  type: 'bank' | 'credit';
  bank_name?: string;
  name: string;
  card_no4?: string;
  expire_month?: number;
  expire_year?: number;
  credit_limit?: number;
  balance?: number;
}

<CardFace
  acc={...}
  flipped={!!flipped[acc.id]}
  onFlip={() => flip(acc)}
  fullInfo={fullInfoMap[acc.id] ?? null}  // { full_card_no?, cvv? }
  loadingFull={!!loadingMap[acc.id]}
/>
```

---

## 三、敏感信息按需解密流程

### 交互原则

- **默认脱敏**：卡号 `**** **** ****1234`，CVV 全 `·`
- **点击触发**：点击卡面（或卡片列表的 👁 图标）→ 前端调 `accountApi.getFullCardNo(id)`
- **按需缓存**：仅当前会话缓存到 `Record<number, { full_card_no, cvv }>` state
- **翻回即清除**：翻回正面不清除缓存（用户可再次翻转查看），离开页面自动释放

### 代码模板

```tsx
const [fullInfoMap, setFullInfoMap] = useState<Record<number, { full_card_no?: string; cvv?: string } | null>>({});
const [loadingMap, setLoadingMap] = useState<Record<number, boolean>>({});
const [flipped, setFlipped] = useState<Record<number, boolean>>({});

const flip = async (acc: Account) => {
  const nowFlipped = flipped[acc.id];
  setFlipped(prev => ({ ...prev, [acc.id]: !prev[acc.id] }));
  if (nowFlipped) return;

  if (fullInfoMap[acc.id] === undefined) {
    setLoadingMap(prev => ({ ...prev, [acc.id]: true }));
    try {
      const data = await accountApi.getFullCardNo(acc.id);
      setFullInfoMap(prev => ({ ...prev, [acc.id]: data || null }));
    } catch {
      setFullInfoMap(prev => ({ ...prev, [acc.id]: null }));
      toast.warning('该卡未保存完整卡号或 CVV');
    } finally {
      setLoadingMap(prev => ({ ...prev, [acc.id]: false }));
    }
  }
};
```

### 完整卡号分组

```ts
function groupCardNo(full: string): string {
  return full.replace(/\D/g, '').slice(0, 16).replace(/(.{4})/g, '$1 ').trim();
}
```

### Luhn 校验（前端提示用）

```ts
function isValidLuhn(num: string): boolean {
  if (!num || !/^\d+$/.test(num)) return false;
  if (num.length < 13 || num.length > 19) return false;
  let sum = 0, shouldDouble = false;
  for (let i = num.length - 1; i >= 0; i--) {
    let d = parseInt(num[i], 10);
    if (shouldDouble) { d *= 2; if (d > 9) d -= 9; }
    sum += d; shouldDouble = !shouldDouble;
  }
  return sum % 10 === 0;
}
```

---

## 四、银行名 Combobox

### POPULAR_BANKS 常量（28 家）

```ts
const POPULAR_BANKS = [
  '招商银行', '中国工商银行', '中国建设银行', '中国农业银行', '中国银行',
  '交通银行', '浦发银行', '中信银行', '兴业银行', '民生银行',
  '光大银行', '平安银行', '华夏银行', '广发银行', '浙商银行',
  '渤海银行', '恒丰银行', '中国邮政储蓄银行', '北京银行', '上海银行',
  '宁波银行', '江苏银行', '南京银行', '杭州银行', '微众银行',
  '网商银行', '新网银行', '其他',
];
```

### BankCombobox 组件要点

- `<input>` onFocus / onChange 时 setOpen(true)
- 弹层顶部 `<div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />` 做 click-outside
- 搜索过滤：`POPULAR_BANKS.filter(b => b.toLowerCase().includes(value.toLowerCase()))`
- 同时支持自定义输入（不限制必须从列表选）

---

## 五、表单字段 Checklist

储蓄卡 & 信用卡通用：

| 字段 | 前端类型 | 备注 |
|------|---------|------|
| bank_name | BankCombobox | — |
| full_card_no | text + 自动分组 | 每 4 位空格，onChange 同步 card_no4 |
| card_no4 | text + maxLength=4 | 从完整卡号自动填充 |

信用卡专属：

| 字段 | 前端类型 | 备注 |
|------|---------|------|
| cvv | `type="password"` + maxLength=4 | 加密存 |
| expire_month | number + maxLength=2 | 两位，左补零 |
| expire_year | number + maxLength=2 | 两位数（27 表示 2027）|
| credit_limit | number | — |
| bill_day / repay_day | number 1-31 | — |

---

## 六、文件修改地图

```
新增敏感字段时：
backend/internal/models/models.go         → +EncryptedXXX + json:"-"
backend/internal/dto/dto.go             → +XXX string binding:"omitempty,max=N"
backend/internal/handlers/account.go     → Create/Update 加密 + GetFullCardNo 返回
backend/internal/router/router.go       → 路由不变（复用 :id/full-card）
frontend/src/types/index.ts             → 加非加密字段类型
frontend/src/pages/accounts/AccountsPage.tsx → blankForm + openEdit + submitEdit + UI
frontend/src/pages/cards/CardsPage.tsx  → CardFaceData + CardFace 渲染
frontend/src/styles/index.css           → 如有新卡面样式追加
frontend/src/api/index.ts               → accountApi.getFullCardNo 已存在
```

---

## 七、安全红线

1. **任何加密字段禁止加 `json` tag 返回默认值**（必须 `json:"-"`）
2. **GetFullCardNo 必须校验 user_id**，不能只按 id 查
3. **前端 CVV 输入必须 `type="password"`**
4. **Edit 时敏感字段非空才覆盖**，避免更新其他字段时误刷空
5. **密钥统一用 JWT.Secret**，不搞多密钥管理
6. **前端完整卡号/CVV 仅存组件 state**，不写 localStorage / store

---

## 八、新增卡功能示例

想加「身份证号」加密字段？

1. models.go: `EncryptedIdentity string gorm:"size:512" json:"-"`
2. dto.go: `Identity string binding:"omitempty,max=20"`
3. account.go Create/Update: `if req.Identity != "" { updates["encrypted_identity"] = encryptCardNo(req.Identity) }`
4. GetFullCardNo: 加 `"identity": decryptCardNo(acc.EncryptedIdentity)`
5. 前端表单加 `type="password"` 输入框 + 加密提示
6. 卡片 / 卡面按需解密显示（仅眼睛按钮触发）
