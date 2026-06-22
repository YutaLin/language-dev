use std::fs::File;
use std::io::{BufRead, BufReader};
use anyhow::Result;

pub struct CsvReader {
    reader: BufReader<File>,
}

impl CsvReader {
    pub fn open(path: &str) -> Result<Self> {
        let file = File::open(path)?;
        let reader = BufReader::new(file);
        Ok(CsvReader{ reader })
    }
}

impl Iterator for CsvReader {
    type Item = Result<Vec<String>>;

    fn next(&mut self) -> Option<Self::Item> {
        let mut line = String::new();
        match self.reader.read_line(&mut line) {
            Ok(0) => None,
            Ok(_) => {
                let trimmed = line.trim_end();
                let fields: Vec<String> = trimmed
                    .split(",")
                    .map(|s| s.to_string())
                    .collect();
                Some(Ok(fields))
            }
            Err(e) => Some(Err(e.into())),
        }
    }
}

fn main() -> Result<()> {
    let csv = CsvReader::open("sample.csv")?;

    for row in csv {
        let row = row?;
        println!("{:?}", row);
    }

    Ok(())
}
