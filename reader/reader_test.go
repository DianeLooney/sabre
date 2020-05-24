package reader

import (
	"bytes"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spy16/sabre/core"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		r        io.Reader
		fileName string
	}{
		{
			name:     "WithStringReader",
			r:        strings.NewReader(":test"),
			fileName: "<string>",
		},
		{
			name:     "WithBytesReader",
			r:        bytes.NewReader([]byte(":test")),
			fileName: "<bytes>",
		},
		{
			name:     "WihFile",
			r:        os.NewFile(0, "test.lisp"),
			fileName: "test.lisp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rd := New(tt.r)
			if rd == nil {
				t.Errorf("New() should return instance of Reader, got nil")
			} else if rd.File != tt.fileName {
				t.Errorf("core.File = \"%s\", want = \"%s\"", rd.File, tt.name)
			}
		})
	}
}

func TestReader_SetMacro(t *testing.T) {
	t.Run("UnsetDefaultMacro", func(t *testing.T) {
		rd := New(strings.NewReader("~hello"))
		rd.SetMacro('~', nil, false) // remove unquote operator

		var want core.Value
		want = core.Symbol{
			Value: "~hello",
			Position: core.Position{
				File:   "<string>",
				Line:   1,
				Column: 1,
			},
		}

		got, err := rd.One()
		if err != nil {
			t.Errorf("unexpected error: %#v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %#v, want = %#v", got, want)
		}
	})

	t.Run("CustomMacro", func(t *testing.T) {
		rd := New(strings.NewReader("~hello"))
		rd.SetMacro('~', func(rd *Reader, _ rune) (core.Value, error) {
			var ru []rune
			for {
				r, err := rd.NextRune()
				if err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				}

				if rd.IsTerminal(r) {
					break
				}
				ru = append(ru, r)
			}

			return core.String(ru), nil
		}, false) // override unquote operator

		var want core.Value
		want = core.String("hello")

		got, err := rd.One()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got = %v, want = %v", got, want)
		}
	})
}

func TestReader_All(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		want    core.Value
		wantErr bool
	}{
		{
			name: "ValidLiteralSample",
			src:  `'hello #{} 123 "Hello\tWorld" 12.34 -0xF +010 true nil 0b1010 \a :hello`,
			want: core.Module{
				&core.List{
					Values: []core.Value{
						core.Symbol{Value: "quote"},
						core.Symbol{
							Value: "hello",
							Position: core.Position{
								File:   "<string>",
								Line:   1,
								Column: 2,
							},
						},
					},
				},
				core.Set{
					Position: core.Position{
						File:   "<string>",
						Line:   1,
						Column: 9,
					},
				},
				core.Int64(123),
				core.String("Hello\tWorld"),
				core.Float64(12.34),
				core.Int64(-15),
				core.Int64(8),
				core.Bool(true),
				core.Nil{},
				core.Int64(10),
				core.Character('a'),
				core.Keyword("hello"),
			},
		},
		{
			name: "WithComment",
			src:  `:valid-keyword ; comment should return errSkip`,
			want: core.Module{core.Keyword("valid-keyword")},
		},
		{
			name:    "UnterminatedString",
			src:     `:valid-keyword "unterminated string literal`,
			wantErr: true,
		},
		{
			name: "CommentFollowedByForm",
			src:  `; comment should return errSkip` + "\n" + `:valid-keyword`,
			want: core.Module{core.Keyword("valid-keyword")},
		},
		{
			name:    "UnterminatedList",
			src:     `:valid-keyword (add 1 2`,
			wantErr: true,
		},
		{
			name:    "UnterminatedVector",
			src:     `:valid-keyword [1 2`,
			wantErr: true,
		},
		{
			name:    "EOFAfterQuote",
			src:     `:valid-keyword '`,
			wantErr: true,
		},
		{
			name:    "CommentAfterQuote",
			src:     `:valid-keyword ';hello world`,
			wantErr: true,
		},
		{
			name:    "UnbalancedParenthesis",
			src:     `())`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(strings.NewReader(tt.src)).All()
			if (err != nil) != tt.wantErr {
				t.Errorf("All() error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("All() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestReader_One(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name:    "Empty",
			src:     "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "QuotedEOF",
			src:     "';comment is a no-op form\n",
			wantErr: true,
		},
		{
			name:    "ListEOF",
			src:     "( 1",
			wantErr: true,
		},
		{
			name: "UnQuote",
			src:  "~(x 3)",
			want: &core.List{
				Values: []core.Value{
					core.Symbol{Value: "unquote"},
					&core.List{
						Values: []core.Value{
							core.Symbol{
								Value: "x",
								Position: core.Position{
									File:   "<string>",
									Line:   1,
									Column: 3,
								},
							},
							core.Int64(3),
						},
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
				},
			},
		},
	})
}

func TestReader_One_Number(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "NumberWithLeadingSpaces",
			src:  "    +1234",
			want: core.Int64(1234),
		},
		{
			name: "PositiveInt",
			src:  "+1245",
			want: core.Int64(1245),
		},
		{
			name: "NegativeInt",
			src:  "-234",
			want: core.Int64(-234),
		},
		{
			name: "PositiveFloat",
			src:  "+1.334",
			want: core.Float64(1.334),
		},
		{
			name: "NegativeFloat",
			src:  "-1.334",
			want: core.Float64(-1.334),
		},
		{
			name: "PositiveHex",
			src:  "0x124",
			want: core.Int64(0x124),
		},
		{
			name: "NegativeHex",
			src:  "-0x124",
			want: core.Int64(-0x124),
		},
		{
			name: "PositiveOctal",
			src:  "0123",
			want: core.Int64(0123),
		},
		{
			name: "NegativeOctal",
			src:  "-0123",
			want: core.Int64(-0123),
		},
		{
			name: "PositiveBinary",
			src:  "0b10",
			want: core.Int64(2),
		},
		{
			name: "NegativeBinary",
			src:  "-0b10",
			want: core.Int64(-2),
		},
		{
			name: "PositiveBase2Radix",
			src:  "2r10",
			want: core.Int64(2),
		},
		{
			name: "NegativeBase2Radix",
			src:  "-2r10",
			want: core.Int64(-2),
		},
		{
			name: "PositiveBase4Radix",
			src:  "4r123",
			want: core.Int64(27),
		},
		{
			name: "NegativeBase4Radix",
			src:  "-4r123",
			want: core.Int64(-27),
		},
		{
			name: "ScientificSimple",
			src:  "1e10",
			want: core.Float64(1e10),
		},
		{
			name: "ScientificNegativeExponent",
			src:  "1e-10",
			want: core.Float64(1e-10),
		},
		{
			name: "ScientificWithDecimal",
			src:  "1.5e10",
			want: core.Float64(1.5e+10),
		},
		{
			name:    "FloatStartingWith0",
			src:     "012.3",
			want:    core.Float64(012.3),
			wantErr: false,
		},
		{
			name:    "InvalidValue",
			src:     "1ABe13",
			wantErr: true,
		},
		{
			name:    "InvalidScientificFormat",
			src:     "1e13e10",
			wantErr: true,
		},
		{
			name:    "InvalidExponent",
			src:     "1e1.3",
			wantErr: true,
		},
		{
			name:    "InvalidRadixFormat",
			src:     "1r2r3",
			wantErr: true,
		},
		{
			name:    "RadixBase3WithDigit4",
			src:     "-3r1234",
			wantErr: true,
		},
		{
			name:    "RadixMissingValue",
			src:     "2r",
			wantErr: true,
		},
		{
			name:    "RadixInvalidBase",
			src:     "2ar",
			wantErr: true,
		},
		{
			name:    "RadixWithFloat",
			src:     "2.3r4",
			wantErr: true,
		},
		{
			name:    "DecimalPointInBinary",
			src:     "0b1.0101",
			wantErr: true,
		},
		{
			name:    "InvalidDigitForOctal",
			src:     "08",
			wantErr: true,
		},
		{
			name:    "IllegalNumberFormat",
			src:     "9.3.2",
			wantErr: true,
		},
	})
}

func TestReader_One_String(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "SimpleString",
			src:  `"hello"`,
			want: core.String("hello"),
		},
		{
			name: "EscapeQuote",
			src:  `"double quote is \""`,
			want: core.String(`double quote is "`),
		},
		{
			name: "EscapeSlash",
			src:  `"hello\\world"`,
			want: core.String(`hello\world`),
		},
		{
			name:    "UnexpectedEOF",
			src:     `"double quote is`,
			wantErr: true,
		},
		{
			name:    "InvalidEscape",
			src:     `"hello \x world"`,
			wantErr: true,
		},
		{
			name:    "EscapeEOF",
			src:     `"hello\`,
			wantErr: true,
		},
	})
}

func TestReader_One_Keyword(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "SimpleASCII",
			src:  `:test`,
			want: core.Keyword("test"),
		},
		{
			name: "LeadingTrailingSpaces",
			src:  "          :test          ",
			want: core.Keyword("test"),
		},
		{
			name: "SimpleUnicode",
			src:  `:∂`,
			want: core.Keyword("∂"),
		},
		{
			name: "WithSpecialChars",
			src:  `:this-is-valid?`,
			want: core.Keyword("this-is-valid?"),
		},
		{
			name: "FollowedByMacroChar",
			src:  `:this-is-valid'hello`,
			want: core.Keyword("this-is-valid"),
		},
	})
}

func TestReader_One_Character(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "ASCIILetter",
			src:  `\a`,
			want: core.Character('a'),
		},
		{
			name: "ASCIIDigit",
			src:  `\1`,
			want: core.Character('1'),
		},
		{
			name: "Unicode",
			src:  `\∂`,
			want: core.Character('∂'),
		},
		{
			name: "Newline",
			src:  `\newline`,
			want: core.Character('\n'),
		},
		{
			name: "FormFeed",
			src:  `\formfeed`,
			want: core.Character('\f'),
		},
		{
			name: "Unicode",
			src:  `\u00AE`,
			want: core.Character('®'),
		},
		{
			name:    "InvalidUnicode",
			src:     `\uHELLO`,
			wantErr: true,
		},
		{
			name:    "OutOfRangeUnicode",
			src:     `\u-100`,
			wantErr: true,
		},
		{
			name:    "UnknownSpecial",
			src:     `\hello`,
			wantErr: true,
		},
		{
			name:    "EOF",
			src:     `\`,
			wantErr: true,
		},
	})
}

func TestReader_One_Symbol(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "SimpleASCII",
			src:  `hello`,
			want: core.Symbol{
				Value: "hello",
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "Unicode",
			src:  `find-∂`,
			want: core.Symbol{
				Value: "find-∂",
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "SingleChar",
			src:  `+`,
			want: core.Symbol{
				Value: "+",
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
	})
}

func TestReader_One_List(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "EmptyList",
			src:  `()`,
			want: &core.List{
				Values: nil,
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "ListWithOneEntry",
			src:  `(help)`,
			want: &core.List{
				Values: []core.Value{
					core.Symbol{
						Value: "help",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "ListWithMultipleEntry",
			src:  `(+ 0xF 3.1413)`,
			want: &core.List{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "ListWithCommaSeparator",
			src:  `(+,0xF,3.1413)`,
			want: &core.List{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "MultiLine",
			src: `(+
                      0xF
                      3.1413
					)`,
			want: &core.List{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "MultiLineWithComments",
			src: `(+         ; plus operator adds numerical values
                      0xF    ; hex representation of 15
                      3.1413 ; value of math constant pi
                  )`,
			want: &core.List{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name:    "UnexpectedEOF",
			src:     "(+ 1 2 ",
			wantErr: true,
		},
	})
}

func TestReader_One_Vector(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "Empty",
			src:  `[]`,
			want: core.Vector{
				Values: nil,
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "WithOneEntry",
			src:  `[help]`,
			want: core.Vector{
				Values: []core.Value{
					core.Symbol{
						Value: "help",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "WithMultipleEntry",
			src:  `[+ 0xF 3.1413]`,
			want: core.Vector{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "WithCommaSeparator",
			src:  `[+,0xF,3.1413]`,
			want: core.Vector{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "MultiLine",
			src: `[+
                      0xF
                      3.1413
					]`,
			want: core.Vector{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name: "MultiLineWithComments",
			src: `[+         ; plus operator adds numerical values
                      0xF    ; hex representation of 15
                      3.1413 ; value of math constant pi
                  ]`,
			want: core.Vector{
				Values: []core.Value{
					core.Symbol{
						Value: "+",
						Position: core.Position{
							File:   "<string>",
							Line:   1,
							Column: 2,
						},
					},
					core.Int64(15),
					core.Float64(3.1413),
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 1,
				},
			},
		},
		{
			name:    "UnexpectedEOF",
			src:     "[+ 1 2 ",
			wantErr: true,
		},
	})
}

func TestReader_One_Set(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "Empty",
			src:  "#{}",
			want: core.Set{
				Values: nil,
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 2,
				},
			},
		},
		{
			name: "Valid",
			src:  "#{1 2 []}",
			want: core.Set{
				Values: []core.Value{core.Int64(1),
					core.Int64(2),
					core.Vector{
						Position: core.Position{
							File:   "<string>",
							Column: 7,
							Line:   1,
						},
					},
				},
				Position: core.Position{
					File:   "<string>",
					Line:   1,
					Column: 2,
				},
			},
		},
		{
			name:    "HasDuplicate",
			src:     "#{1 2 2}",
			wantErr: true,
		},
	})
}

func TestReader_One_HashMap(t *testing.T) {
	executeReaderTests(t, []readerTestCase{
		{
			name: "SimpleKeywordMap",
			src: `{:age 10
				   :name "Bob"}`,
			want: &core.HashMap{
				Position: core.Position{File: "<string>", Line: 1, Column: 1},
				Data: map[core.Value]core.Value{
					core.Keyword("age"):  core.Int64(10),
					core.Keyword("name"): core.String("Bob"),
				},
			},
		},
		{
			name:    "NonHashableKey",
			src:     `{[] 10}`,
			wantErr: true,
		},
		{
			name:    "OddNumberOfForms",
			src:     "{:hello 10 :age}",
			wantErr: true,
		},
	})
}

type readerTestCase struct {
	name    string
	src     string
	want    core.Value
	wantErr bool
}

func executeReaderTests(t *testing.T, tests []readerTestCase) {
	t.Parallel()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := New(strings.NewReader(tt.src)).One()
			if (err != nil) != tt.wantErr {
				t.Errorf("One() error = %#v, wantErr %#v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("One() got = %#v, want %#v", got, tt.want)
			}
		})
	}
}