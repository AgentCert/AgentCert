from docx import Document


def main() -> None:
    path = "/mnt/d/AgentCert_Solution_Architecture_Document.docx"
    doc = Document(path)

    for table_index, table in enumerate(doc.tables):
        print(f"TABLE {table_index} rows={len(table.rows)} cols={len(table.columns)}")
        for row_index, row in enumerate(table.rows):
            values = [cell.text.replace("\n", " ").strip() for cell in row.cells]
            print(f"R{row_index}: " + " | ".join(values))
        print("---")


if __name__ == "__main__":
    main()