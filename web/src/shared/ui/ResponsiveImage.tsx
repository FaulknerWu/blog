import type { Image } from "~/shared/api/types";

interface Props {
  image: Image;
  sizes: string;
  className?: string;
  priority?: boolean;
}

// ResponsiveImage 按格式输出多尺寸 <source>，并写入原图宽高以避免布局偏移。
export function ResponsiveImage({ image, sizes, className, priority = false }: Props) {
  const byFormat = Map.groupBy(image.variants, (v) => v.format);
  return (
    <picture>
      {[...byFormat].map(([format, variants]) => (
        <source
          key={format}
          type={`image/${format}`}
          sizes={sizes}
          srcSet={variants.map((v) => `${v.url} ${v.width}w`).join(", ")}
        />
      ))}
      <img
        src={image.src}
        alt={image.alt}
        width={image.width}
        height={image.height}
        className={className}
        loading={priority ? "eager" : "lazy"}
        fetchPriority={priority ? "high" : undefined}
        decoding="async"
      />
    </picture>
  );
}
