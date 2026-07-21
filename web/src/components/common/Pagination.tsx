import { t } from "../../i18n";

export const SEARCH_PAGE_SIZE = 12;

export function Pagination({
  page,
  pageSize = SEARCH_PAGE_SIZE,
  totalItems,
  onPageChange,
}: {
  page: number;
  pageSize?: number;
  totalItems: number;
  onPageChange: (page: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize));
  if (totalPages <= 1) return null;

  const currentPage = Math.min(Math.max(page, 1), totalPages);
  const pages = compactPages(currentPage, totalPages);

  return (
    <nav className="pagination" aria-label={t("pagination.label")}>
      <button
        className="pagination__arrow"
        type="button"
        disabled={currentPage === 1}
        onClick={() => onPageChange(currentPage - 1)}
      >
        {t("pagination.previous")}
      </button>

      <div className="pagination__pages">
        {pages.map((item, index) =>
          item === "ellipsis" ? (
            <span className="pagination__ellipsis" aria-hidden="true" key={`ellipsis-${index}`}>
              …
            </span>
          ) : (
            <button
              className={"pagination__page" + (item === currentPage ? " pagination__page--active" : "")}
              type="button"
              aria-current={item === currentPage ? "page" : undefined}
              aria-label={`${t("pagination.page")} ${item}`}
              key={item}
              onClick={() => onPageChange(item)}
            >
              {item}
            </button>
          ),
        )}
      </div>

      <button
        className="pagination__arrow"
        type="button"
        disabled={currentPage === totalPages}
        onClick={() => onPageChange(currentPage + 1)}
      >
        {t("pagination.next")}
      </button>

      <span className="pagination__summary">
        {t("pagination.page")} {currentPage} {t("pagination.of")} {totalPages}
      </span>
    </nav>
  );
}

type CompactPage = number | "ellipsis";

export function compactPages(currentPage: number, totalPages: number): CompactPage[] {
  const visible = new Set([1, totalPages, currentPage - 1, currentPage, currentPage + 1]);
  const pageNumbers = [...visible]
    .filter((page) => page >= 1 && page <= totalPages)
    .sort((a, b) => a - b);
  const result: CompactPage[] = [];

  pageNumbers.forEach((page, index) => {
    if (index > 0 && page - pageNumbers[index - 1] > 1) result.push("ellipsis");
    result.push(page);
  });

  return result;
}
