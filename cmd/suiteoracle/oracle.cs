// oracle runs T5Suite's, T7Suite's or T8Suite's own symbol table parser on binaries and
// prints every symbol, for TestFactorySuiteSymbols to compare against:
//
//	FILE <path>
//	SYM  <index> <name> <number> <flash address hex> <SRAM address hex> <length> <type hex>
//
// Build it with mono and run it from a copy of the suite's bin/x86/Release
// folder (the parser needs the DLLs next to it), then strip the directories
// from the FILE lines and store the output in testdata/suite/{t5,t7,t8}.tsv:
//
//	mcs -out:oracle.exe oracle.cs
//	cd T5Suite2.0/bin/x86/Release && mono oracle.exe t5 Trionic5Tools.dll .../testdata/T5Binaries/*.BIN > t5.tsv
//	cd T7Suite/bin/x86/Release && mono oracle.exe t7 T7.exe .../testdata/T7Binaries/*.bin > t7.tsv
//	cd T8Suite/bin/x86/Release && mono oracle.exe t8 T8SuitePro.exe .../testdata/T8Binaries/*.BIN > t8.tsv
using System;
using System.Collections;
using System.Reflection;

class Oracle {
    static object Get(object o, string p) {
        var pi = o.GetType().GetProperty(p);
        return pi == null ? "" : pi.GetValue(o, null);
    }

    static int Main(string[] args) {
        string mode = args[0];
        var asm = Assembly.LoadFrom(args[1]);
        string cls = mode == "t5" ? "Trionic5Tools.Trionic5File" : mode == "t7" ? "T7.Trionic7File" : "T8SuitePro.Trionic8File";
        Type type = asm.GetType(cls);
        for (int a = 2; a < args.Length; a++) {
            string bin = args[a];
            // a fresh parser per file: T5Suite's keeps adding to one collection
            object inst = mode == "t8" ? null : Activator.CreateInstance(type);
            Console.WriteLine("FILE\t" + bin);
            try {
                IEnumerable col;
                if (mode == "t5") {
                    type.GetMethod("SelectFile").Invoke(inst, new object[] { bin });
                    object info = type.GetMethod("ParseFile").Invoke(inst, null);
                    col = (IEnumerable)info.GetType().GetProperty("SymbolCollection").GetValue(info, null);
                } else if (mode == "t7") {
                    col = (IEnumerable)type.GetMethod("ExtractFile").Invoke(inst, new object[] { bin, 44, "" });
                } else {
                    object[] p = new object[] { bin, null, null, null };
                    type.GetMethod("TryToExtractPackedBinary").Invoke(null, p);
                    col = (IEnumerable)p[1];
                }
                int i = 0;
                if (col != null) foreach (object s in col) {
                    Console.WriteLine(string.Format("SYM\t{0}\t{1}\t{2}\t{3:X}\t{4:X}\t{5}\t{6:X}", i++,
                        Get(s, "Varname"), Get(s, "Symbol_number"), Get(s, "Flash_start_address"),
                        Get(s, "Start_address"), Get(s, "Length"), Get(s, "Symbol_type")));
                }
            } catch (Exception e) {
                Exception x = e.InnerException ?? e;
                Console.WriteLine("ERR\t" + x.GetType().Name + ": " + x.Message.Replace("\n", " "));
            }
        }
        return 0;
    }
}
