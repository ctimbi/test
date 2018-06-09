import { NgModule } from '@angular/core';
import { FacebookLoginComponent } from './facebook-login/facebook-login';
import { UserLogoutComponent } from './user-logout/user-logout';
@NgModule({
	declarations: [FacebookLoginComponent,
    UserLogoutComponent],
	imports: [],
	exports: [FacebookLoginComponent,
    UserLogoutComponent]
})
export class ComponentsModule {}
