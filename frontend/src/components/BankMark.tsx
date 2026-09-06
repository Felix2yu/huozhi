import { BANK_LOGOS, type BankLogoData } from '@/constants/bankLogos';
import { bankBadge, resolveBankBrand } from '@/constants/banks';

/**
 * 银行标识（纯徽标方案）：
 * 1. 品牌表命中且矢量数据为 emblem 模式 → 白底圆标内嵌裁剪后的银行徽标
 *    （徽标自带官方配色，白底保证在卡面渐变背景上清晰）；
 * 2. 矢量数据为 full 模式（徽标与文字无法分离，如交通银行）→ 白底圆角横条
 *    内嵌完整横版锁标，按内容宽高比缩放；
 * 3. 无矢量数据 → 品牌色 / 哈希色圆标 + 品牌短字（bankBadge）。
 * text 无法解析时返回 null，由调用方决定兜底（如账户类型 emoji）。
 */
export function BankMark({ text, size = 40, className }: {
  text?: string;
  size?: number;
  className?: string;
}) {
  const brand = resolveBankBrand(text);
  const logo = brand ? BANK_LOGOS[brand.icon ?? ''] : undefined;
  if (brand && logo) {
    if (logo.mode === 'stroke') {
      // 描边风格图标：整体按品牌色着色
      return (
        <div
          className={cnMerge('rounded-full bg-white grid place-items-center shrink-0 shadow-sm', className)}
          style={{ width: size, height: size, color: brand.color }}
          title={brand.name}
        >
          <svg
            width={Math.round(size * 0.62)}
            height={Math.round(size * 0.62)}
            viewBox={logo.crop.join(' ')}
            xmlns="http://www.w3.org/2000/svg"
            aria-label={brand.name}
            dangerouslySetInnerHTML={{ __html: logo.body ?? '' }}
          />
        </div>
      );
    }
    if (logo.mode === 'mono') {
      // 单色字形：品牌色着色，白底圆标
      return (
        <LogoSvg
          data={{ ...logo, paths: logo.paths.map(p => ({ ...p, fill: brand.color })) }}
          boxClassName={cnMerge('rounded-full bg-white shadow-sm', className)}
          boxStyle={{ width: size, height: size }}
          inner={Math.round(size * 0.6)}
          title={brand.name}
        />
      );
    }
    if (logo.mode === 'emblem') {
      return (
        <LogoSvg
          data={logo}
          boxClassName={cnMerge('rounded-full bg-white shadow-sm', className)}
          boxStyle={{ width: size, height: size }}
          inner={Math.round(size * 0.72)}
          title={brand.name}
        />
      );
    }
    // full 模式：横版锁标按宽高比铺在圆角横条里
    const ratio = logo.crop[2] / logo.crop[3];
    const h = Math.round(size * 0.56);
    const w = Math.min(Math.round(size * 1.5), Math.round(h * ratio));
    return (
      <LogoSvg
        data={logo}
        boxClassName={cnMerge('rounded-full bg-white shadow-sm', className)}
        boxStyle={{ width: w, height: h }}
        inner={Math.round(h * 0.8)}
        title={brand.name}
      />
    );
  }
  const fb = bankBadge(text);
  if (!fb) return null;
  return (
    <div
      className={cnMerge('rounded-full grid place-items-center font-bold text-white shrink-0 shadow-sm', className)}
      style={{ width: size, height: size, background: fb.color, fontSize: size * 0.42 }}
      title={fb.name}
    >
      {fb.short}
    </div>
  );
}

function LogoSvg({ data, boxClassName, boxStyle, inner, title }: {
  data: BankLogoData;
  boxClassName: string;
  boxStyle: React.CSSProperties;
  inner: number;
  title: string;
}) {
  const [x, y, w, h] = data.crop;
  return (
    <div
      className={cnMerge('grid place-items-center shrink-0 overflow-hidden', boxClassName)}
      style={boxStyle}
      title={title}
    >
      <svg width={inner} height={inner} viewBox={`${x} ${y} ${w} ${h}`} xmlns="http://www.w3.org/2000/svg" aria-label={title}>
        {data.paths.map((p, i) => <path key={i} fill={p.fill} d={p.d} />)}
      </svg>
    </div>
  );
}

// 避免为这一个用法引入整个 cn 工具时的循环依赖：本地简单拼接
function cnMerge(...parts: (string | false | undefined | null)[]): string {
  return parts.filter(Boolean).join(' ');
}
