import { Component, Input } from '@angular/core';

@Component({
  selector: "procinfo",
  templateUrl: './procinfo.component.html',
  styleUrl: './procinfo.component.css',
})
export class ProcinfoComponent {
  @Input({ required: true }) title!: string;
  @Input() entries: string[] | undefined;
  @Input() sorted = false;

  displayTitle(): string {
    if (this.title.length > 12) {
      return this.title.substring(12) + '...';
    }
    return this.title;
  }

  displayDescription(): string {
    const entries = this.displayEntries();
    if (entries.length === 0) {
      return '';
    }
    return entries[0];
  }

  displayEntries(): string[] {
    if (!this.entries) {
      return [];
    }
    if (this.sorted) {
      return this.entries.sort((a: string, b: string) => a.localeCompare(b));
    }
    return this.entries;
  }
}
